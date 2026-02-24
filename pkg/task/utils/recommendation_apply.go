package utils

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

// ApplyCheckInput holds all inputs for deciding whether to apply recommendations to a pod.
// Used by both the apply-recommendation task and the admission webhook.
type ApplyCheckInput struct {
	ApplyBlacklistedNamespaces []string
	K8sVersionGE133            bool
	K8sMemoryGE134             bool
	OptimizeGuaranteedPods     bool
	DisableMemoryApplication   bool
	NewWorkloadThresholdHours  int
	SkipMemory                 bool // task metadata skip memory (e.g. when cluster < 1.34)
	PodExcludedByAnnotation    bool // when true, treat as excluded (from pod.Annotations or workload constraints)
}

// ShouldGenerateRecommendation returns true if recommendations should be applied to this pod.
// podForExclusion: when non-nil (webhook path), pod name and annotations are used for prefix/annotation checks.
// When nil (task path), podInfo.Name and podInfo.Stats.Constraints.ExcludedAnnotation are used.
func ShouldGenerateRecommendation(
	ctx context.Context,
	podInfo *PodInfo,
	input ApplyCheckInput,
	podForExclusion *corev1.Pod,
) (bool, string) {
	if len(input.ApplyBlacklistedNamespaces) > 0 && slices.Contains(input.ApplyBlacklistedNamespaces, podInfo.Namespace) {
		return false, "namespace is blacklisted"
	}

	if input.PodExcludedByAnnotation {
		return false, "pod annotation is excluded"
	}

	if podInfo.Stats == nil {
		return false, "no stats for workload"
	}

	if podInfo.IsGuaranteedPod() && !input.OptimizeGuaranteedPods {
		return false, "guaranteed pod and config disables optimizing guaranteed pods"
	}
	if podInfo.IsBestEffortPod() {
		return false, "best effort pod"
	}

	if podInfo.Stats.CreationTime.After(time.Now().Add(-1 * time.Hour * time.Duration(input.NewWorkloadThresholdHours))) {
		return false, "workload is newer than NewWorkloadThresholdHours"
	}

	if podInfo.Stats.IsHorizontallyAutoscaledOnCPU || podInfo.Stats.IsHorizontallyAutoscaledOnMem {
		return false, "workload is horizontally autoscaled on CPU or memory"
	}

	return true, ""
}

func ShouldApplyRecommendationToPod(
	ctx context.Context,
	podInfo *PodInfo,
	override *types.WorkloadOverrideInfo,
	input ApplyCheckInput,
	podForExclusion *corev1.Pod,
) (bool, string) {
	apply, reason := ShouldGenerateRecommendation(ctx, podInfo, input, podForExclusion)
	if !apply {
		return false, reason
	}

	if !input.K8sVersionGE133 {
		return false, "kubernetes version is not v1.33 or above"
	}

	if override == nil || !override.EffectiveEnabled() {
		return false, fmt.Sprintf("cruisekube not enabled for workload %s (no override or recommend-only mode), skipping apply", podInfo.Stats.WorkloadIdentifier)
	}

	return true, ""
}

// ComputeRecommendedResourceValues returns recommended CPU request, memory request, CPU limit, memory limit
// for a container recommendation. allocatableCPU is used for CPU limit (e.g. node allocatable or a default).
func ComputeRecommendedResourceValues(ctx context.Context, rec PodContainerRecommendation, allocatableCPU float64) (float64, float64, float64, float64) {
	cpuRequest := EnforceMinimumCPU(rec.CPU)
	if cpuRequest > CPUClampValue {
		cpuRequest = CPUClampValue
	}
	memoryRequest := EnforceMinimumMemory(rec.Memory)
	// We add allocatableCPU as the default cpu limit, as if a pod is running, we CAN'T remove the cpu limit.
	cpuLimit := allocatableCPU
	memoryLimit := memoryRequest * 2
	if rec.PodInfo.Stats != nil {
		if containerStat, err := rec.PodInfo.Stats.GetContainerStats(rec.ContainerName); err == nil {
			var memMax, oom float64
			if containerStat.Memory7Day != nil {
				memMax = containerStat.Memory7Day.Max
			}
			if containerStat.MemoryStats != nil && containerStat.MemoryStats.OOMMemory > 0 {
				oom = containerStat.MemoryStats.OOMMemory
			}
			memoryLimit = EnforceMinimumMemory(max(memMax, oom) * 2)
		}
	} else {
		logging.Warnf(ctx, "No stats for container %s", rec.ContainerName)
	}
	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

// PodExcludedByAnnotation returns true if the pod has the cruisekube excluded annotation.
func PodExcludedByAnnotation(pod interface{}) bool {
	switch p := pod.(type) {
	case *corev1.Pod:
		return p.Annotations[ExcludedAnnotation] == TrueValue
	case *corev1.PodTemplateSpec:
		return p.Annotations[ExcludedAnnotation] == TrueValue
	default:
		return false
	}
}
