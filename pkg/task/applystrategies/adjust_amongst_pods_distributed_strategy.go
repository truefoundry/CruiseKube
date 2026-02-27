package applystrategies

import (
	"context"
	"math"
	"sort"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"

	"k8s.io/client-go/kubernetes"
)

func NewAdjustAmongstPodsDistributedStrategy(ctx context.Context) AdjustAmongstPodsDistributedStrategy {
	return AdjustAmongstPodsDistributedStrategy{}
}

type AdjustAmongstPodsDistributedStrategy struct {
}

func (s *AdjustAmongstPodsDistributedStrategy) GetName() string {
	return "AdjustAmongstPodsDistributed"
}

func (s *AdjustAmongstPodsDistributedStrategy) OptimizeNode(ctx context.Context, kubeClient *kubernetes.Clientset, overridesMap map[string]*types.WorkloadOverrideInfo, data utils.NodeOptimizationData) (utils.OptimizationResult, error) {
	result := utils.OptimizationResult{
		PodContainerRecommendations: make([]utils.PodContainerRecommendation, 0),
		MaxRestCPU:                  0,
		MaxRestMemory:               0,
	}

	podInfosClone := make([]utils.PodInfo, len(data.PodInfos))
	copy(podInfosClone, data.PodInfos)

	podMetricsCache := make(map[string]utils.PodMetrics)

	// Calculate the recommendation for each pod
	for _, podInfo := range podInfosClone {
		podKey := utils.GetPodKey(podInfo.Namespace, podInfo.Name)
		var totalRecommendedCPU, totalRecommendedMemory, maxRestCPU, maxRestMemory float64
		for _, containerRec := range podInfo.Stats.ContainerStats {
			if containerRec.ContainerType == types.InitContainer {
				continue
			}
			recommendedCPU, cpuRest := s.GetRecommendedAndRestCPU(ctx, podInfo, containerRec)
			recommendedMemory, memoryRest := s.GetRecommendedAndRestMemory(ctx, podInfo, containerRec)

			if podInfo.WorkloadKind == utils.DaemonSetKind {
				containerResource, err := podInfo.GetContainerResource(containerRec.ContainerName)
				if err != nil {
					logging.Errorf(ctx, "Error getting container resource for container %s: %v", containerRec.ContainerName, err)
					continue
				}
				currentCPU := containerResource.CPURequest
				currentMemory := containerResource.MemoryRequest

				if recommendedCPU > currentCPU {
					// We don't want to increase the CPU request for daemonsets
					// as they might end up in a state that not all daemonsets will fit in a single node
					recommendedCPU = currentCPU
				}
				cpuRest = 0

				if recommendedMemory > currentMemory {
					recommendedMemory = currentMemory
				}
				memoryRest = 0
			}
			totalRecommendedCPU += recommendedCPU
			maxRestCPU = math.Max(maxRestCPU, cpuRest)
			totalRecommendedMemory += recommendedMemory
			maxRestMemory = math.Max(maxRestMemory, memoryRest)
		}

		var maxInitCPU, maxInitMemory float64
		if podInfo.Stats != nil && podInfo.Stats.OriginalContainerResources != nil {
			for _, containerRes := range podInfo.Stats.OriginalContainerResources {
				if containerRes.Type == types.InitContainer {
					maxInitCPU = max(maxInitCPU, containerRes.CPURequest)
					maxInitMemory = max(maxInitMemory, containerRes.MemoryRequest)
				}
			}
		}

		evictionRanking := types.EvictionRankingHigh
		if podInfo.Stats.EvictionRanking != 0 {
			evictionRanking = podInfo.Stats.EvictionRanking
		}
		overrides, ok := overridesMap[podInfo.Stats.WorkloadIdentifier]
		if ok && overrides.EffectiveEvictionRanking() != 0 {
			evictionRanking = overrides.EffectiveEvictionRanking()
		}

		podMetricsCache[podKey] = utils.PodMetrics{
			TotalRecommendedCPU:    max(totalRecommendedCPU, maxInitCPU),
			TotalRecommendedMemory: max(totalRecommendedMemory, maxInitMemory),
			MaxRestCPU:             maxRestCPU,
			MaxRestMemory:          maxRestMemory,
			EvictionRanking:        evictionRanking,
		}
	}

	// Sort and perform eviction based on memory
	sort.Slice(podInfosClone, func(i, j int) bool {
		if podInfosClone[i].WorkloadKind == utils.DaemonSetKind && podInfosClone[j].WorkloadKind != utils.DaemonSetKind {
			return false
		}
		if podInfosClone[i].WorkloadKind != utils.DaemonSetKind && podInfosClone[j].WorkloadKind == utils.DaemonSetKind {
			return true
		}
		podKey_i := utils.GetPodKey(podInfosClone[i].Namespace, podInfosClone[i].Name)
		podKey_j := utils.GetPodKey(podInfosClone[j].Namespace, podInfosClone[j].Name)
		metrics_i := podMetricsCache[podKey_i]
		metrics_j := podMetricsCache[podKey_j]

		if metrics_i.EvictionRanking < metrics_j.EvictionRanking {
			return false
		}
		if metrics_i.EvictionRanking > metrics_j.EvictionRanking {
			return true
		}
		return metrics_i.MaxRestMemory > metrics_j.MaxRestMemory
	})
	podInfosClone = s.performEvictionLoop(podInfosClone, podMetricsCache, data.AllocatableMemory,
		func(metrics utils.PodMetrics) float64 { return metrics.TotalRecommendedMemory },
		func(metrics utils.PodMetrics) float64 { return metrics.MaxRestMemory },
		&result)

	// Sort and perform eviction based on CPU
	sort.Slice(podInfosClone, func(i, j int) bool {
		if podInfosClone[i].WorkloadKind == utils.DaemonSetKind && podInfosClone[j].WorkloadKind != utils.DaemonSetKind {
			return false
		}
		if podInfosClone[i].WorkloadKind != utils.DaemonSetKind && podInfosClone[j].WorkloadKind == utils.DaemonSetKind {
			return true
		}

		podKey_i := utils.GetPodKey(podInfosClone[i].Namespace, podInfosClone[i].Name)
		podKey_j := utils.GetPodKey(podInfosClone[j].Namespace, podInfosClone[j].Name)

		metrics_i := podMetricsCache[podKey_i]
		metrics_j := podMetricsCache[podKey_j]

		if metrics_i.EvictionRanking < metrics_j.EvictionRanking {
			return false
		}
		if metrics_i.EvictionRanking > metrics_j.EvictionRanking {
			return true
		}

		return metrics_i.MaxRestCPU > metrics_j.MaxRestCPU
	})
	podInfosClone = s.performEvictionLoop(podInfosClone, podMetricsCache, data.AllocatableCPU,
		func(metrics utils.PodMetrics) float64 { return metrics.TotalRecommendedCPU },
		func(metrics utils.PodMetrics) float64 { return metrics.MaxRestCPU },
		&result)

	maxRestCPU := 0.0
	maxRestMemory := 0.0
	totalRestCPU := 0.0
	totalRestMemory := 0.0
	totalRecommendedCPU := 0.0
	totalRecommendedMemory := 0.0

	containerMetrics := make([]struct {
		pod               utils.PodInfo
		containerStats    utils.ContainerStats
		currentCPU        float64
		currentMemory     float64
		restCPU           float64
		restMemory        float64
		recommendedCPU    float64
		recommendedMemory float64
	}, 0)

	for _, pod := range podInfosClone {
		if pod.Stats.ContainerStats != nil {
			for _, containerStat := range pod.Stats.ContainerStats {
				if containerStat.ContainerType == types.InitContainer {
					continue
				}
				currentResource, err := pod.GetContainerResource(containerStat.ContainerName)
				if err != nil {
					logging.Errorf(ctx, "Error getting container resource for container %s: %v", containerStat.ContainerName, err)
					continue
				}
				currentCPU := currentResource.CPURequest
				currentMemory := currentResource.MemoryRequest

				recommendedCPU, restCPU := s.GetRecommendedAndRestCPU(ctx, pod, containerStat)
				recommendedMemory, restMemory := s.GetRecommendedAndRestMemory(ctx, pod, containerStat)

				if pod.WorkloadKind == utils.DaemonSetKind {
					currentCPU := currentResource.CPURequest
					currentMemory := currentResource.MemoryRequest

					if recommendedCPU > currentCPU {
						// We don't want to increase the CPU request for daemonsets
						// as they might end up in a state that not all daemonsets will fit in a single node
						recommendedCPU = currentCPU
					}
					restCPU = 0

					if recommendedMemory > currentMemory {
						recommendedMemory = currentMemory
					}
					restMemory = 0
				}

				containerMetrics = append(containerMetrics, struct {
					pod               utils.PodInfo
					containerStats    utils.ContainerStats
					currentCPU        float64
					currentMemory     float64
					restCPU           float64
					restMemory        float64
					recommendedCPU    float64
					recommendedMemory float64
				}{
					pod:               pod,
					containerStats:    containerStat,
					currentCPU:        currentCPU,
					currentMemory:     currentMemory,
					restCPU:           restCPU,
					restMemory:        restMemory,
					recommendedCPU:    recommendedCPU,
					recommendedMemory: recommendedMemory,
				})

				maxRestCPU = math.Max(maxRestCPU, restCPU)
				maxRestMemory = math.Max(maxRestMemory, restMemory)
				totalRestCPU += restCPU
				totalRestMemory += restMemory
				totalRecommendedCPU += recommendedCPU
				totalRecommendedMemory += recommendedMemory
			}
		}
	}

	// s.logNodeMaxRestResources(podMetricsCache, data.NodeName)

	for _, metric := range containerMetrics {
		var additionalCPU, additionalMemory float64

		if totalRecommendedCPU > 0 && totalRestCPU > 0 {
			cpuRatio := metric.restCPU / totalRestCPU
			additionalCPU = maxRestCPU * cpuRatio
		}

		if totalRecommendedMemory > 0 && totalRestMemory > 0 {
			memoryRatio := metric.restMemory / totalRestMemory
			additionalMemory = maxRestMemory * memoryRatio
		}

		finalCPU := metric.recommendedCPU + additionalCPU
		finalMemory := metric.recommendedMemory + additionalMemory

		logging.Infof(ctx, "Distributed strategy for %s/%s/%s: base_cpu=%.3f, additional_cpu=%.3f, final_cpu=%.3f, base_memory=%.3f, additional_memory=%.3f, final_memory=%.3f",
			metric.pod.Namespace, metric.pod.Name, metric.containerStats.ContainerName,
			metric.recommendedCPU, additionalCPU, finalCPU, metric.recommendedMemory, additionalMemory, finalMemory)

		podContainerRec := utils.PodContainerRecommendation{
			PodInfo:       metric.pod,
			ContainerName: metric.containerStats.ContainerName,
			CPU:           finalCPU,
			Memory:        finalMemory,
			Evict:         false,
		}
		result.PodContainerRecommendations = append(result.PodContainerRecommendations, podContainerRec)
	}

	for _, pod := range podInfosClone {
		if pod.Stats.ContainerStats == nil {
			logging.Errorf(ctx, "No container recommendations found for pod %s/%s", pod.Namespace, pod.Name)
		}
	}

	result.MaxRestCPU = maxRestCPU
	result.MaxRestMemory = maxRestMemory

	return result, nil
}

func (s *AdjustAmongstPodsDistributedStrategy) performEvictionLoop(
	podInfosClone []utils.PodInfo,
	podMetricsCache map[string]utils.PodMetrics,
	allocatableResource float64,
	getTotalRecommended func(utils.PodMetrics) float64,
	getMaxRest func(utils.PodMetrics) float64,
	result *utils.OptimizationResult,
) []utils.PodInfo {
	i := 0
	for i < len(podInfosClone) {
		idealConsumption := 0.0
		idealRequiredRest := 0.0
		for _, p := range podInfosClone {
			podKey := utils.GetPodKey(p.Namespace, p.Name)
			metrics := podMetricsCache[podKey]
			idealConsumption += getTotalRecommended(metrics)
			idealRequiredRest = math.Max(getMaxRest(metrics), idealRequiredRest)
		}

		spare := allocatableResource - idealConsumption
		if idealConsumption <= allocatableResource && idealRequiredRest <= spare {
			break
		}

		podInfo := podInfosClone[i]
		evictionRanking := podMetricsCache[utils.GetPodKey(podInfo.Namespace, podInfo.Name)].EvictionRanking

		if isEvictionExcludedPod(&podInfo, evictionRanking) {
			i++
			continue
		}

		for _, containerStat := range podInfo.Stats.ContainerStats {
			if containerStat.ContainerType == types.InitContainer {
				continue
			}
			result.PodContainerRecommendations = append(result.PodContainerRecommendations, utils.PodContainerRecommendation{
				PodInfo:       podInfo,
				ContainerName: containerStat.ContainerName,
				CPU:           containerStat.SimplePredictionsCPU.MaxValue,
				Memory:        containerStat.SimplePredictionsMemory.MaxValue,
				Evict:         true,
			})
		}

		podInfosClone = append(podInfosClone[:i], podInfosClone[i+1:]...)
	}
	return podInfosClone
}

func (s *AdjustAmongstPodsDistributedStrategy) GetRecommendedAndRestMemory(ctx context.Context, pod utils.PodInfo, containerStat utils.ContainerStats) (float64, float64) {
	if containerStat.MemoryStats.OOMMemory > 0 && containerStat.MemoryStats.OOMMemory > containerStat.MemoryStats.P75 {
		logging.Infof(ctx, "Using OOM memory for %s/%s/%s: %v", pod.Namespace, pod.Name, containerStat.ContainerName, containerStat.MemoryStats.OOMMemory)
		// We are not underestimating the memory requests here, we are just using the OOM memory as the total recommended memory with no extra headspace
		return containerStat.MemoryStats.OOMMemory, 0.0
	}
	if containerStat.SimplePredictionsMemory == nil {
		logging.Errorf(ctx, "Error: No simple predictions found for %s/%s/%s", pod.Namespace, pod.Name, containerStat.ContainerName)
		return containerStat.MemoryStats.P75, containerStat.MemoryStats.Max - containerStat.MemoryStats.P75
	} else {
		return containerStat.MemoryStats.P75, containerStat.SimplePredictionsMemory.MaxValue - containerStat.MemoryStats.P75
	}
}

func (s *AdjustAmongstPodsDistributedStrategy) GetRecommendedAndRestCPU(ctx context.Context, pod utils.PodInfo, containerStat utils.ContainerStats) (float64, float64) {
	recommendedCPU := containerStat.CPUStats.P75
	if containerStat.PSIAdjustedUsage != nil {
		recommendedCPU = containerStat.PSIAdjustedUsage.P75
	}

	pmax := containerStat.CPUStats.Max
	if containerStat.SimplePredictionsCPU != nil {
		pmax = containerStat.SimplePredictionsCPU.MaxValue
	}

	rest := (pmax - recommendedCPU)

	logging.Infof(ctx, "Variable diff calculation for %s/%s/%s: recommended_cpu=%.1f, pmax=%.3f, rest=%.3f",
		pod.Namespace, pod.Name, containerStat.ContainerName,
		recommendedCPU, pmax, rest)

	return recommendedCPU, rest
}
