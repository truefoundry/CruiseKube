package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

type mutatingPatchInput struct {
	pod *corev1.Pod
}

type mutatingPatchResolvedContext struct {
	pod          *corev1.Pod
	clients      *cluster.ClusterClients
	workloadInfo *utils.WorkloadInfo
	workloadKey  string
	stat         *types.WorkloadStat
	overrides    *types.Overrides
}

type mutatingPatchResult struct {
	statusCode   int
	errorMessage string
	patches      []map[string]any
	audit        *mutatingPatchResolvedContext
}

func (deps HandlerDependencies) HandleMutatingPatch(c *gin.Context) {
	clusterID := c.Param("clusterID")
	ctx := contextutils.WithCluster(c.Request.Context(), clusterID)

	var req client.MutatingPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deps.writeMutatingPatchResponse(c, clusterID, deps.evaluateMutatingPatch(ctx, clusterID, req))
}

func (deps HandlerDependencies) evaluateMutatingPatch(ctx context.Context, clusterID string, req client.MutatingPatchRequest) mutatingPatchResult {
	input, result := extractMutatingPatchInput(ctx, req)
	if result != nil {
		return *result
	}

	resolved, result := deps.resolveMutatingPatchContext(ctx, clusterID, input.pod)
	if result != nil {
		return *result
	}

	if !deps.shouldApplyMutatingPatch(ctx, clusterID, resolved) {
		return emptyMutatingPatchResult()
	}

	patches, err := deps.buildMutatingPatches(ctx, clusterID, resolved)
	if err != nil {
		logging.Errorf(ctx, "Failed to adjust resources for pod %s/%s: %v", resolved.pod.Namespace, getPodName(resolved.pod), err)
		return emptyMutatingPatchResult()
	}

	return mutatingPatchResult{
		statusCode: http.StatusOK,
		patches:    patches,
		audit:      resolved,
	}
}

func extractMutatingPatchInput(ctx context.Context, req client.MutatingPatchRequest) (*mutatingPatchInput, *mutatingPatchResult) {
	review := req.Review
	if review.Request == nil {
		logging.Warnf(ctx, "Admission review has no request")
		return nil, &mutatingPatchResult{
			statusCode: http.StatusOK,
			patches:    emptyPatchList(),
		}
	}

	// Only mutate Pods.
	if review.Request.Kind.Kind != "Pod" {
		logging.Warnf(ctx, "Admission review request is not a Pod, skipping")
		return nil, &mutatingPatchResult{
			statusCode: http.StatusOK,
			patches:    emptyPatchList(),
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		logging.Errorf(ctx, "Failed to decode pod from admission request: %v", err)
		return nil, &mutatingPatchResult{
			statusCode:   http.StatusBadRequest,
			errorMessage: "invalid pod object",
		}
	}

	return &mutatingPatchInput{pod: &pod}, nil
}

func (deps HandlerDependencies) resolveMutatingPatchContext(ctx context.Context, clusterID string, pod *corev1.Pod) (*mutatingPatchResolvedContext, *mutatingPatchResult) {
	workloadInfo := utils.GetWorkloadInfoFromPod(pod)
	if workloadInfo == nil {
		logging.Infof(ctx, "Pod %s/%s has no workload owner, skipping recommendation", pod.Namespace, pod.Name)
		return nil, &mutatingPatchResult{
			statusCode: http.StatusOK,
			patches:    emptyPatchList(),
		}
	}

	clients, err := deps.ClusterManager.GetClusterClients(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get cluster clients for %s: %v", clusterID, err)
		return nil, &mutatingPatchResult{
			statusCode: http.StatusOK,
			patches:    emptyPatchList(),
		}
	}

	workloadKey := utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)

	stat, err := deps.Storage.GetStatForWorkload(clusterID, workloadKey)
	if errors.Is(err, storage.ErrWorkloadNotFound) {
		logging.Infof(ctx, "No stats for workload %s yet, skipping patch for pod %s/%s", workloadKey, pod.Namespace, pod.Name)
		return nil, &mutatingPatchResult{
			statusCode: http.StatusOK,
			patches:    emptyPatchList(),
		}
	}
	if err != nil {
		logging.Errorf(ctx, "Failed to get stat for workload %s: %v", workloadKey, err)
		return nil, &mutatingPatchResult{
			statusCode: http.StatusOK,
			patches:    emptyPatchList(),
		}
	}

	overrides, err := deps.Storage.GetWorkloadOverrides(clusterID, workloadKey)
	if err != nil {
		logging.Warnf(ctx, "Failed to get workload overrides for %s in cluster %s: %v; proceeding without overrides", workloadKey, clusterID, err)
	}

	return &mutatingPatchResolvedContext{
		pod:          pod,
		clients:      clients,
		workloadInfo: workloadInfo,
		workloadKey:  workloadKey,
		stat:         stat,
		overrides:    overrides,
	}, nil
}

func (deps HandlerDependencies) shouldApplyMutatingPatch(ctx context.Context, clusterID string, resolved *mutatingPatchResolvedContext) bool {
	cfg := deps.Config
	overrideInfo := buildWorkloadOverrideInfo(resolved.workloadKey, resolved.stat, resolved.overrides)
	podInfo := utils.BuildPodInfoFromPod(resolved.pod, resolved.workloadInfo, resolved.stat)
	input := utils.ApplyCheckInput{
		ApplyBlacklistedNamespaces: cfg.RecommendationSettings.ApplyBlacklistedNamespaces,
		K8sVersionGE133:            utils.CheckIfClusterVersionAbove(ctx, clusterID, resolved.clients.KubeClient, 1, 33),
		K8sMemoryGE134:             utils.CheckIfClusterVersionAbove(ctx, clusterID, resolved.clients.KubeClient, 1, 34),
		OptimizeGuaranteedPods:     cfg.RecommendationSettings.OptimizeGuaranteedPods,
		DisableMemoryApplication:   cfg.RecommendationSettings.DisableMemoryApplication,
		NewWorkloadThresholdHours:  cfg.RecommendationSettings.NewWorkloadThresholdHours,
		SkipMemory:                 false,
		PodExcludedByAnnotation:    utils.PodExcludedByAnnotation(resolved.pod),
	}

	apply, reason := utils.ShouldApplyRecommendationToPod(ctx, &podInfo, overrideInfo, input)
	if !apply {
		logging.Infof(ctx, "Skipping recommendation for pod %s/%s: %s", resolved.pod.Namespace, getPodName(resolved.pod), reason)
	}
	return apply
}

func (deps HandlerDependencies) buildMutatingPatches(ctx context.Context, clusterID string, resolved *mutatingPatchResolvedContext) ([]map[string]any, error) {
	patches, err := deps.adjustResources(ctx, resolved.pod, clusterID, resolved.workloadInfo, resolved.stat)
	if err != nil {
		return nil, err
	}

	disruptionPatches := buildDisruptionAnnotationPatches(ctx, resolved.pod, resolved.stat, resolved.overrides)
	patches = append(patches, disruptionPatches...)
	if patches == nil {
		return emptyPatchList(), nil
	}
	return patches, nil
}

func (deps HandlerDependencies) writeMutatingPatchResponse(c *gin.Context, clusterID string, result mutatingPatchResult) {
	ctx := contextutils.WithCluster(c.Request.Context(), clusterID)
	if result.errorMessage != "" {
		c.JSON(result.statusCode, gin.H{"error": result.errorMessage})
		return
	}

	if len(result.patches) > 0 && deps.AuditRecorder != nil && result.audit != nil {
		message := fmt.Sprintf("Pod %s/%s mutated with disruption annotation changes", result.audit.pod.Namespace, getPodName(result.audit.pod))
		if hasResourcePatch(result.patches) {
			message = fmt.Sprintf("Pod %s/%s mutated with resource recommendations", result.audit.pod.Namespace, getPodName(result.audit.pod))
		}
		deps.AuditRecorder.Record(ctx, clusterID, types.AuditEvent{
			Type:     types.EventTypeNormal,
			Category: types.EventCategoryWebhookMutation,
			Payload: types.AuditPayload{
				Message: message,
				Target:  map[string]interface{}{"kind": result.audit.pod.Kind, "namespace": result.audit.pod.Namespace, "name": getPodName(result.audit.pod)},
				Details: map[string]interface{}{
					"workloadId": result.audit.workloadKey,
					"node":       result.audit.pod.Spec.NodeName,
					"patches":    result.patches,
				},
			},
		})
	}

	c.JSON(result.statusCode, result.patches)
}

func emptyMutatingPatchResult() mutatingPatchResult {
	return mutatingPatchResult{
		statusCode: http.StatusOK,
		patches:    emptyPatchList(),
	}
}

func emptyPatchList() []map[string]any {
	return []map[string]any{}
}

func hasResourcePatch(patches []map[string]any) bool {
	for _, patch := range patches {
		path, _ := patch["path"].(string)
		if strings.Contains(path, "/resources/requests") || strings.Contains(path, "/resources/limits") {
			return true
		}
	}
	return false
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

// Keep this helper on HandlerDependencies because it still needs injected services
// from the webhook flow, and moving it off the receiver would reintroduce globals
// or add avoidable plumbing parameters.
func (deps HandlerDependencies) adjustResources(ctx context.Context, pod *corev1.Pod, clusterID string, workloadInfo *utils.WorkloadInfo, workloadStat *types.WorkloadStat) ([]map[string]any, error) {
	cfg := deps.Config
	if workloadInfo == nil {
		workloadInfo = utils.GetWorkloadInfoFromPod(pod)
	}
	if workloadInfo == nil {
		logging.Warnf(ctx, "Could not determine workload for pod %s/%s, allowing without adjustment", pod.Namespace, getPodName(pod))
		return []map[string]any{}, nil
	}

	logging.Infof(ctx, "Pod %s/%s belongs to workload: %s", pod.Namespace, getPodName(pod), utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name))

	if workloadStat == nil {
		workloadID := utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)
		var err error
		workloadStat, err = deps.Storage.GetStatForWorkload(clusterID, workloadID)
		if errors.Is(err, storage.ErrWorkloadNotFound) {
			logging.Infof(ctx, "No stat found for workload %s yet, skipping patch", workloadID)
			return []map[string]any{}, nil
		}
		if err != nil {
			logging.Errorf(ctx, "Failed to get stat for workload %s: %v", workloadID, err)
			return []map[string]any{}, nil
		}
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
	err := applyTaskConfig.ConvertMetadataToStruct(&metadata)
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
