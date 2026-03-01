package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/audit"
	"github.com/truefoundry/cruisekube/pkg/client"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func HandleMutatingPatch(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	ctx = contextutils.WithCluster(ctx, clusterID)

	var req client.MutatingPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	review := req.Review
	if review.Request == nil {
		logging.Warnf(ctx, "Admission review has no request")
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	// Only mutate Pods
	if review.Request.Kind.Kind != "Pod" {
		logging.Warnf(ctx, "Admission review request is not a Pod, skipping")
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	var pod corev1.Pod
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		logging.Errorf(ctx, "Failed to decode pod from admission request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pod object"})
		return
	}

	cfg := config.GetConfigFromGinContext(c)
	mgr := c.MustGet("clusterManager").(cluster.Manager)
	clients, err := mgr.GetClusterClients(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get cluster clients for %s: %v", clusterID, err)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	workloadInfo := utils.GetWorkloadInfoFromPod(&pod)
	if workloadInfo == nil {
		logging.Infof(ctx, "Pod %s/%s has no workload owner, skipping recommendation", pod.Namespace, pod.Name)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	workloadKey := utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)

	stat, err := storage.Stg.GetStatForWorkload(clusterID, workloadKey)
	if err != nil {
		logging.Errorf(ctx, "Failed to get stat for workload %s: %v", workloadKey, err)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}
	if stat == nil {
		logging.Infof(ctx, "No stats for workload %s, skipping", workloadKey)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	overrides, _ := storage.Stg.GetWorkloadOverrides(clusterID, workloadKey)
	overrideInfo := buildWorkloadOverrideInfo(workloadKey, stat, overrides)

	podInfo := utils.BuildPodInfoFromPod(&pod, workloadInfo, stat)
	k8sGE133 := utils.CheckIfClusterVersionAbove(ctx, clusterID, clients.KubeClient, 1, 33)
	k8sMemoryGE134 := utils.CheckIfClusterVersionAbove(ctx, clusterID, clients.KubeClient, 1, 34)
	input := utils.ApplyCheckInput{
		ApplyBlacklistedNamespaces: cfg.RecommendationSettings.ApplyBlacklistedNamespaces,
		K8sVersionGE133:            k8sGE133,
		K8sMemoryGE134:             k8sMemoryGE134,
		OptimizeGuaranteedPods:     cfg.RecommendationSettings.OptimizeGuaranteedPods,
		DisableMemoryApplication:   cfg.RecommendationSettings.DisableMemoryApplication,
		NewWorkloadThresholdHours:  cfg.RecommendationSettings.NewWorkloadThresholdHours,
		SkipMemory:                 false,
		PodExcludedByAnnotation:    utils.PodExcludedByAnnotation(&pod),
	}

	apply, reason := utils.ShouldApplyRecommendationToPod(ctx, &podInfo, overrideInfo, input)
	if !apply {
		logging.Infof(ctx, "Skipping recommendation for pod %s/%s: %s", pod.Namespace, getPodName(&pod), reason)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	patches, err := adjustResources(ctx, &pod, clusterID, cfg)
	if err != nil {
		logging.Errorf(ctx, "Failed to adjust resources for pod %s/%s: %v", pod.Namespace, getPodName(&pod), err)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	disruptionPatches := buildDisruptionAnnotationPatches(ctx, &pod, stat, overrides)
	patches = append(patches, disruptionPatches...)

	if len(patches) > 0 && audit.Recorder != nil {
		audit.Recorder.Record(ctx, clusterID, types.AuditEvent{
			Type:     types.EventTypeNormal,
			Category: types.EventCategoryWebhookMutation,
			Payload: types.AuditPayload{
				Message: fmt.Sprintf("Pod %s/%s mutated with resource recommendations", pod.Namespace, getPodName(&pod)),
				Target:  map[string]interface{}{"kind": pod.Kind, "namespace": pod.Namespace, "name": getPodName(&pod)},
				Details: map[string]interface{}{
					"workloadId": workloadKey,
					"node":       pod.Spec.NodeName,
					"patches":    patches,
				},
			},
		})
	}

	c.JSON(http.StatusOK, patches)
}

func getPodName(pod *corev1.Pod) string {
	if pod.Name != "" {
		return pod.Name
	}
	if pod.GenerateName != "" {
		return pod.GenerateName
	}
	return "unknown"
}

func buildWorkloadOverrideInfo(workloadID string, stat *types.WorkloadStat, overrides *types.Overrides) *types.WorkloadOverrideInfo {
	effective := &types.WorkloadOverridesEffective{
		Enabled:         true,
		EvictionRanking: types.EvictionRankingMedium,
	}
	if stat != nil {
		effective.EvictionRanking = stat.EvictionRanking
	}
	if overrides != nil {
		if overrides.Enabled != nil {
			effective.Enabled = *overrides.Enabled
		}
		if overrides.EvictionRanking != nil {
			effective.EvictionRanking = *overrides.EvictionRanking
		}
	}
	name, ns, kind := "", "", ""
	if stat != nil {
		name, ns, kind = stat.Name, stat.Namespace, stat.Kind
	}
	return &types.WorkloadOverrideInfo{
		WorkloadID: workloadID,
		Name:       name,
		Namespace:  ns,
		Kind:       kind,
		Overrides:  effective,
	}
}

func adjustResources(ctx context.Context, pod *corev1.Pod, clusterID string, cfg *config.Config) ([]map[string]any, error) {
	workloadInfo := utils.GetWorkloadInfoFromPod(pod)
	if workloadInfo == nil {
		logging.Warnf(ctx, "Could not determine workload for pod %s/%s, allowing without adjustment", pod.Namespace, getPodName(pod))
		return []map[string]any{}, nil
	}

	logging.Infof(ctx, "Pod %s/%s belongs to workload: %s", pod.Namespace, getPodName(pod), utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name))

	workloadID := utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)
	workloadStat, err := storage.Stg.GetStatForWorkload(clusterID, workloadID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get stat for workload %s: %v", workloadID, err)
		return []map[string]any{}, nil
	}

	containers := make([]corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	containers = append(containers, pod.Spec.Containers...)
	containers = append(containers, pod.Spec.InitContainers...)
	var patches []map[string]any
	for i, container := range containers {
		containerPath := fmt.Sprintf("/spec/containers/%d", i)
		if i >= len(pod.Spec.Containers) {
			containerPath = fmt.Sprintf("/spec/initContainers/%d", i-len(pod.Spec.Containers))
		}

		if container.Resources.Requests == nil {
			patches = append(patches, map[string]any{
				"op":    "add",
				"path":  containerPath + "/resources/requests",
				"value": map[string]string{},
			})
		}
		if container.Resources.Limits == nil {
			patches = append(patches, map[string]any{
				"op":    "add",
				"path":  containerPath + "/resources/limits",
				"value": map[string]string{},
			})
		}

		if container.Resources.Limits != nil {
			if _, exists := container.Resources.Limits[corev1.ResourceCPU]; exists {
				patches = append(patches, map[string]any{
					"op":   "remove",
					"path": containerPath + "/resources/limits/cpu",
				})
			}
		}

		var containerStat *utils.ContainerStats
		for _, stat := range workloadStat.ContainerStats {
			if stat.ContainerName == container.Name {
				containerStat = &stat
				break
			}
		}

		if containerStat == nil || containerStat.CPUStats == nil || containerStat.MemoryStats == nil || containerStat.SimplePredictionsCPU == nil || containerStat.SimplePredictionsMemory == nil {
			logging.Infof(ctx, "No stat found for container: %s in workload: %s/%s/%s", container.Name, workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)
			continue
		}

		recommendedCPU := containerStat.CPUStats.Max
		if containerStat.SimplePredictionsCPU != nil && containerStat.SimplePredictionsCPU.MaxValue > 0 {
			recommendedCPU = containerStat.SimplePredictionsCPU.MaxValue
		}
		if recommendedCPU > utils.CPUClampValue {
			recommendedCPU = utils.CPUClampValue
		}

		recommendedMemory := containerStat.MemoryStats.Max
		if containerStat.MemoryStats.OOMMemory > 0 && containerStat.MemoryStats.OOMMemory > containerStat.MemoryStats.Max {
			recommendedMemory = containerStat.MemoryStats.OOMMemory
		} else if containerStat.SimplePredictionsMemory != nil && containerStat.SimplePredictionsMemory.MaxValue > 0 {
			recommendedMemory = containerStat.SimplePredictionsMemory.MaxValue
		}

		recommendedMemoryLimit := 2 * recommendedMemory
		if containerStat.Memory7Day != nil && containerStat.Memory7Day.Max > 0 {
			recommendedMemoryLimit = math.Max(2*containerStat.Memory7Day.Max, max(2*containerStat.MemoryStats.OOMMemory, recommendedMemoryLimit))
		}

		recommendedMemoryLimitBytes := int64(math.Max(recommendedMemoryLimit, 512) * utils.BytesToMBDivisor)

		logging.Infof(ctx, "Container %s - Recommended CPU: %s (max: %f)", container.Name, cpuCoresToMillicores(recommendedCPU), containerStat.CPUStats.Max)
		logging.Infof(ctx, "Container %s - Recommended Memory: %s", container.Name, memoryBytesToMB(int64(recommendedMemory*utils.BytesToMBDivisor)))

		var currentCPURequest resource.Quantity
		var currentMemoryRequest resource.Quantity
		var currentMemoryLimit resource.Quantity

		if container.Resources.Requests != nil {
			currentCPURequest = container.Resources.Requests[corev1.ResourceCPU]
			currentMemoryRequest = container.Resources.Requests[corev1.ResourceMemory]
		}

		if container.Resources.Limits != nil {
			currentMemoryLimit = container.Resources.Limits[corev1.ResourceMemory]
		}

		// CPU
		currentCPUMillicores := currentCPURequest.MilliValue()
		recommendedCPUMillicores := math.Max(float64(recommendedCPU*1000), 1)

		if currentCPUMillicores > 0 {
			if workloadInfo.Kind != utils.DaemonSetKind {
				patches = append(patches, map[string]any{
					"op":    "replace",
					"path":  containerPath + "/resources/requests/cpu",
					"value": fmt.Sprintf("%dm", int64(recommendedCPUMillicores)),
				})
				logging.Infof(ctx, "Adjusted CPU for container %s from %dm to %dm", container.Name, currentCPUMillicores, int64(recommendedCPUMillicores))
			}
		}

		// Memory
		currentMemoryBytes := currentMemoryRequest.Value()
		recommendedMemoryBytes := int64(recommendedMemory * utils.BytesToMBDivisor)
		thresholdBytes := float64(16 * utils.BytesToMBDivisor)

		if !cfg.RecommendationSettings.DisableMemoryApplication && currentMemoryBytes > 0 && math.Abs(float64(recommendedMemoryBytes-currentMemoryBytes)) > thresholdBytes {
			if workloadInfo.Kind == utils.DaemonSetKind {
				if currentMemoryLimit.Value() > 0 {
					currentMemoryLimitBytes := currentMemoryLimit.Value()
					finalMemoryLimitBytes := math.Max(float64(currentMemoryLimitBytes), math.Max(float64(recommendedMemoryLimitBytes), 16*utils.BytesToMBDivisor))

					patches = append(patches, map[string]any{
						"op":    "replace",
						"path":  containerPath + "/resources/limits/memory",
						"value": memoryBytesToMB(int64(finalMemoryLimitBytes)),
					})

					logging.Infof(ctx, "Adjusted Memory limit only for DaemonSet container %s to %s (request unchanged: %dMB)", container.Name, memoryBytesToMB(int64(finalMemoryLimitBytes)), currentMemoryRequest.Value()/utils.BytesToMBDivisor)
				}
			} else {
				if currentMemoryRequest.Value() > 0 {
					patches = append(patches, map[string]any{
						"op":    "replace",
						"path":  containerPath + "/resources/requests/memory",
						"value": memoryBytesToMB(recommendedMemoryBytes),
					})
				} else {
					patches = append(patches, map[string]any{
						"op":    "add",
						"path":  containerPath + "/resources/requests/memory",
						"value": memoryBytesToMB(recommendedMemoryBytes),
					})
				}
				if currentMemoryLimit.Value() > 0 {
					patches = append(patches, map[string]any{
						"op":    "replace",
						"path":  containerPath + "/resources/limits/memory",
						"value": memoryBytesToMB(recommendedMemoryLimitBytes),
					})
				} else {
					patches = append(patches, map[string]any{
						"op":    "add",
						"path":  containerPath + "/resources/limits/memory",
						"value": memoryBytesToMB(recommendedMemoryLimitBytes),
					})
				}

				logging.Infof(ctx, "Adjusted Memory for container %s from %dMB to %dMB", container.Name, currentMemoryRequest.Value()/utils.BytesToMBDivisor, recommendedMemoryBytes/utils.BytesToMBDivisor)
			}
		} else if cfg.RecommendationSettings.DisableMemoryApplication {
			logging.Infof(ctx, "Skipping memory recommendation application for container %s since memory recommendation application is disabled", container.Name)
		}
	}

	metadata := task.ApplyRecommendationMetadata{}
	applyTaskConfig := cfg.GetTaskConfig(config.ApplyRecommendationKey)
	if applyTaskConfig == nil {
		return nil, fmt.Errorf("missing task config for %s", config.ApplyRecommendationKey)
	}
	err = applyTaskConfig.ConvertMetadataToStruct(&metadata)
	if err != nil {
		return nil, fmt.Errorf("convert metadata to struct: %w", err)
	}

	if metadata.DryRun {
		logging.Infof(ctx, "Dry run mode enabled, skipping applying patches")
		return patches, nil
	}

	return patches, nil
}

func memoryBytesToMB(memoryBytes int64) string {
	return fmt.Sprintf("%dM", memoryBytes/(utils.BytesToMBDivisor))
}

func cpuCoresToMillicores(cpuCores float64) string {
	return fmt.Sprintf("%dm", int64(cpuCores*1000))
}

func buildDisruptionAnnotationPatches(ctx context.Context, pod *corev1.Pod, stat *types.WorkloadStat, overrides *types.Overrides) []map[string]any {
	if stat == nil || stat.Constraints == nil || !stat.Constraints.DoNotDisruptAnnotation {
		return nil
	}

	var windows []types.DisruptionWindow
	if overrides != nil {
		windows = overrides.DisruptionWindows
	}
	if len(windows) == 0 {
		return nil
	}

	if !utils.IsInAnyDisruptionWindow(ctx, windows) {
		return nil
	}

	var patches []map[string]any
	for annotationKey := range utils.GetDoNotDisruptAnnotations() {
		if _, exists := pod.Annotations[annotationKey]; !exists {
			continue
		}
		patches = append(patches, map[string]any{
			"op":   "remove",
			"path": "/metadata/annotations/" + escapeJSONPointer(annotationKey),
		})
	}

	if len(patches) > 0 {
		logging.Infof(ctx, "Disruption window active: stripping do-not-disrupt annotations from pod %s/%s", pod.Namespace, getPodName(pod))
		op := "add"
		if pod.Annotations != nil {
			if _, exists := pod.Annotations[utils.AnnotationModified]; exists {
				op = "replace"
			}
		}
		patches = append(patches, map[string]any{
			"op":    op,
			"path":  "/metadata/annotations/" + escapeJSONPointer(utils.AnnotationModified),
			"value": utils.TrueValue,
		})
	}

	return patches
}

// escapeJSONPointer encodes a string for use as a JSON Pointer token (RFC 6901):
// '~' → '~0', '/' → '~1'.
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
