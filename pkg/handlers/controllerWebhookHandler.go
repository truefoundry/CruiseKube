package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/client"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/applystrategies"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

var (
	// ExcludedPodPrefixes is a list of pod name prefixes that are excluded from recommendation application.
	// TODO: load from database/config later.
	ExcludedPodPrefixes = []string{}
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

	dryRun := review.Request.DryRun != nil && *review.Request.DryRun
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
		logging.Infof(ctx, "Pod %s/%s has no workload owner, skipping recommendation", pod.Namespace, utils.GetPodName(&pod))
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
		DryRun:                     dryRun || cfg.Webhook.DryRun,
		ApplyBlacklistedNamespaces: cfg.RecommendationSettings.ApplyBlacklistedNamespaces,
		ExcludedPodPrefixes:        ExcludedPodPrefixes,
		K8sVersionGE133:            k8sGE133,
		K8sMemoryGE134:             k8sMemoryGE134,
		OptimizeGuaranteedPods:     cfg.RecommendationSettings.OptimizeGuaranteedPods,
		DisableMemoryApplication:   cfg.RecommendationSettings.DisableMemoryApplication,
		NewWorkloadThresholdHours:  cfg.RecommendationSettings.NewWorkloadThresholdHours,
		SkipMemory:                 false,
		PodExcludedByAnnotation:    utils.PodExcludedByAnnotation(&pod),
	}

	apply, reason := utils.ShouldApplyRecommendationToPod(ctx, &podInfo, overrideInfo, input, &pod)
	if !apply {
		logging.Infof(ctx, "Skipping recommendation for pod %s/%s: %s", pod.Namespace, getPodName(&pod), reason)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	recommendations, err := applystrategies.ComputeSinglePodRecommendations(podInfo)
	if err != nil {
		logging.Errorf(ctx, "Failed to compute recommendations for pod %s/%s: %v", pod.Namespace, getPodName(&pod), err)
		c.JSON(http.StatusOK, []client.JSONPatchOp{})
		return
	}

	// No node at admission time; use a large default for CPU limit.
	allocatableCPU := float64(0)
	patches := buildPodPatches(&pod, recommendations, allocatableCPU, k8sMemoryGE134, !cfg.RecommendationSettings.DisableMemoryApplication)
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

// buildPodPatches builds RFC 6902 JSON patch operations for the pod from recommendations.
func buildPodPatches(
	pod *corev1.Pod,
	recommendations []utils.PodContainerRecommendation,
	allocatableCPU float64,
	supportsMemoryReduction bool,
	applyMemory bool,
) []client.JSONPatchOp {
	var patches []client.JSONPatchOp
	containerIndexByName := make(map[string]int)
	for i, c := range pod.Spec.Containers {
		containerIndexByName[c.Name] = i
	}

	for _, rec := range recommendations {
		idx, ok := containerIndexByName[rec.ContainerName]
		if !ok {
			continue
		}
		cpuRequest, memoryRequest, cpuLimit, memoryLimit := utils.ComputeRecommendedResourceValues(rec, allocatableCPU)

		basePath := fmt.Sprintf("/spec/containers/%d/resources", idx)

		// Ensure resources block exists
		container := &pod.Spec.Containers[idx]
		if container.Resources.Requests == nil {
			patches = append(patches, client.JSONPatchOp{Op: "add", Path: basePath + "/requests", Value: map[string]interface{}{}})
		}
		if container.Resources.Limits == nil {
			patches = append(patches, client.JSONPatchOp{Op: "add", Path: basePath + "/limits", Value: map[string]interface{}{}})
		}

		// CPU request (replace or add)
		cpuRequestStr := formatCPU(cpuRequest)
		patches = appendPatchOp(patches, container.Resources.Requests != nil && hasCPURequest(container), basePath+"/requests/cpu", cpuRequestStr)

		// CPU limit: 0 means no limit (omit). Otherwise replace/add.
		if cpuLimit > 0 {
			cpuLimitStr := formatCPU(cpuLimit)
			patches = appendPatchOp(patches, container.Resources.Limits != nil && hasCPULimit(container), basePath+"/limits/cpu", cpuLimitStr)
		}

		if applyMemory {
			memoryRequestMB := int64(math.Round(memoryRequest))
			memoryRequestStr := fmt.Sprintf("%dM", memoryRequestMB)
			patches = appendPatchOp(patches, container.Resources.Requests != nil && hasMemoryRequest(container), basePath+"/requests/memory", memoryRequestStr)

			if !supportsMemoryReduction && container.Resources.Limits != nil {
				if currentMemLimit := getContainerMemoryLimitMB(container); currentMemLimit > 0 && memoryLimit < float64(currentMemLimit) {
					memoryLimit = float64(currentMemLimit)
				}
			}
			if memoryLimit > 0 {
				memoryLimitMB := int64(math.Round(memoryLimit))
				memoryLimitStr := fmt.Sprintf("%dM", memoryLimitMB)
				patches = appendPatchOp(patches, container.Resources.Limits != nil && hasMemoryLimit(container), basePath+"/limits/memory", memoryLimitStr)
			}
		}
	}

	return patches
}

func appendPatchOp(patches []client.JSONPatchOp, hasExisting bool, path, value string) []client.JSONPatchOp {
	op := "add"
	if hasExisting {
		op = "replace"
	}
	return append(patches, client.JSONPatchOp{Op: op, Path: path, Value: value})
}

func formatCPU(cpu float64) string {
	return fmt.Sprintf("%dm", int64(cpu*1000))
}

func hasCPURequest(c *corev1.Container) bool {
	if c.Resources.Requests == nil {
		return false
	}
	q, ok := c.Resources.Requests[corev1.ResourceCPU]
	return ok && !q.IsZero()
}
func hasCPULimit(c *corev1.Container) bool {
	if c.Resources.Limits == nil {
		return false
	}
	q, ok := c.Resources.Limits[corev1.ResourceCPU]
	return ok && !q.IsZero()
}
func hasMemoryRequest(c *corev1.Container) bool {
	if c.Resources.Requests == nil {
		return false
	}
	q, ok := c.Resources.Requests[corev1.ResourceMemory]
	return ok && !q.IsZero()
}
func hasMemoryLimit(c *corev1.Container) bool {
	if c.Resources.Limits == nil {
		return false
	}
	q, ok := c.Resources.Limits[corev1.ResourceMemory]
	return ok && !q.IsZero()
}

func getContainerMemoryLimitMB(c *corev1.Container) int64 {
	if c.Resources.Limits == nil {
		return 0
	}
	q := c.Resources.Limits[corev1.ResourceMemory]
	return q.Value() / utils.BytesToMBDivisor
}
