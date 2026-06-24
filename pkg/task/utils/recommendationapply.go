package utils

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

// ApplyCheckInput holds all inputs for deciding whether to apply recommendations to a pod.
// Used by both the apply-recommendation task and the admission webhook.
type ApplyCheckInput struct {
	K8sVersionGE133           bool
	K8sMemoryGE134            bool
	OptimizeGuaranteedPods    bool
	DisableMemoryApplication  bool
	NewWorkloadThresholdHours int
	SkipMemory                bool // task metadata skip memory (e.g. when cluster < 1.34)
	PodExcludedByAnnotation   bool // when true, treat as excluded (from pod.Annotations or workload constraints)
	// HPAResourceAwareOptimization, when true, allows a workload that is
	// horizontally autoscaled on a single resource (CPU or memory) to still be
	// optimized on the other resource, instead of being skipped entirely.
	HPAResourceAwareOptimization bool
}

// ShouldGenerateRecommendation returns true if recommendations should be applied to this pod.
// When nil (task path), podInfo.Name and podInfo.Stats.Constraints.ExcludedAnnotation are used.
func ShouldGenerateRecommendation(
	ctx context.Context,
	podInfo *PodInfo,
	input ApplyCheckInput,
) (bool, string) {
	if podInfo.Stats == nil {
		return false, "no stats for workload"
	}
	if podInfo.IsBestEffortPod() {
		return false, "best effort pod"
	}

	if podInfo.Stats.IsIncomplete() {
		return false, "workload has incomplete stats"
	}

	if podInfo.Stats.CreationTime.After(time.Now().Add(-1 * time.Hour * time.Duration(input.NewWorkloadThresholdHours))) {
		return false, "workload is newer than NewWorkloadThresholdHours"
	}

	hpaOnCPU := podInfo.Stats.IsHorizontallyAutoscaledOnCPU
	hpaOnMem := podInfo.Stats.IsHorizontallyAutoscaledOnMem
	if hpaOnCPU || hpaOnMem {
		// When HPA-resource-aware optimization is enabled, only skip the
		// workload if BOTH CPU and memory are HPA-managed (nothing left to
		// safely right-size). If only one resource is HPA-managed, we still
		// optimize the other resource; the per-resource gates
		// (ShouldApplyCPU/ShouldApplyMemory) ensure we never touch the
		// HPA-managed resource.
		if !input.HPAResourceAwareOptimization || (hpaOnCPU && hpaOnMem) {
			return false, "workload is horizontally autoscaled on CPU or memory"
		}
	}

	return true, ""
}

// ShouldApplyCPU returns false when the CPU request must not be modified for
// this pod because its workload is horizontally autoscaled on CPU (and
// HPA-resource-aware optimization is enabled). Changing the CPU request of an
// HPA-on-CPU workload would skew the utilization signal the HPA scales on.
func ShouldApplyCPU(podInfo *PodInfo, input ApplyCheckInput) bool {
	if podInfo.Stats == nil {
		return true
	}
	return !input.HPAResourceAwareOptimization || !podInfo.Stats.IsHorizontallyAutoscaledOnCPU
}

// ShouldApplyMemory returns false when the memory request must not be modified
// for this pod because its workload is horizontally autoscaled on memory (and
// HPA-resource-aware optimization is enabled).
func ShouldApplyMemory(podInfo *PodInfo, input ApplyCheckInput) bool {
	if podInfo.Stats == nil {
		return true
	}
	return !input.HPAResourceAwareOptimization || !podInfo.Stats.IsHorizontallyAutoscaledOnMem
}

func ShouldApplyRecommendationToPod(
	ctx context.Context,
	podInfo *PodInfo,
	override *types.WorkloadOverrideInfo,
	input ApplyCheckInput,
) (bool, string) {
	shouldGenerate, reason := ShouldGenerateRecommendation(ctx, podInfo, input)
	if !shouldGenerate {
		return false, reason
	}

	if input.PodExcludedByAnnotation {
		return false, "pod annotation is excluded"
	}

	if podInfo.IsGuaranteedPod() && !input.OptimizeGuaranteedPods {
		return false, "guaranteed pod and config disables optimizing guaranteed pods"
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
			// Keep derived memory limit from 7-day/OOM signal, but never let it
			// drop below the chosen memory request, otherwise Kubernetes rejects
			// the pod update (request must be <= limit).
			memoryLimit = max(memoryRequest, EnforceMinimumMemory(max(memMax, oom)*2))
		}
	} else {
		logging.Warnf(ctx, "No stats for container %s", rec.ContainerName)
	}

	cpuRequest = math.Round(cpuRequest*1000) / 1000
	cpuLimit = math.Round(cpuLimit*1000) / 1000
	// If we are setting a CPU limit, ensure it is never below request.
	// Kubernetes validates request <= limit for the same resource.
	if cpuLimit > 0 {
		cpuLimit = max(cpuLimit, cpuRequest)
	}
	memoryRequest = math.Round(memoryRequest)
	memoryLimit = math.Round(memoryLimit)
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
