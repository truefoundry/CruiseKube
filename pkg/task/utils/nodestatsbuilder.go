package utils

import (
	"context"
	"fmt"

	"github.com/truefoundry/cruisekube/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type NodeStatMap map[string]NodeResourceInfo

func CreatePodToStatsMapping(ctx context.Context, podToWorkloadMap map[PodKey]WorkloadInfo, statMap map[string]*WorkloadStat) map[PodKey]*WorkloadStat {
	podStats := make(map[PodKey]*WorkloadStat)

	for podKey, workloadInfo := range podToWorkloadMap {
		identifier := GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)

		var stat *WorkloadStat
		var found bool

		if stat, found = statMap[identifier]; found {
			podStats[podKey] = stat
		} else {
			logging.Warnf(ctx, "No stats found for workload %s", identifier)
		}
	}

	return podStats
}

func CreateNodeStatsMapping(
	ctx context.Context,
	kubeClient *kubernetes.Clientset,
	podStats map[PodKey]*WorkloadStat,
	podToWorkloadMap map[PodKey]WorkloadInfo,
	allPods []*corev1.Pod,
) (map[string]NodeResourceInfo, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	nodeMap := make(NodeStatMap)

	for _, node := range nodes.Items {
		// TODO: can we centralise the check for GPUs?
		if gpuQuantity, exists := node.Status.Allocatable["nvidia.com/gpu"]; exists && gpuQuantity.Value() > 0 {
			logging.Infof(ctx, "Node %s has GPUs, skipping", node.Name)
			continue
		}
		nodeName := node.Name

		allocatableCPUQuantity := node.Status.Allocatable[corev1.ResourceCPU]
		allocatableMemoryQuantity := node.Status.Allocatable[corev1.ResourceMemory]
		allocatableCPU := float64(allocatableCPUQuantity.MilliValue()) / 1000.0
		allocatableMemory := float64(allocatableMemoryQuantity.Value()) / BytesToMBDivisor

		var podsOnNode []*corev1.Pod
		for _, pod := range allPods {
			if pod.Spec.NodeName == nodeName {
				podsOnNode = append(podsOnNode, pod)
			}
		}

		nodeInfo := NodeResourceInfo{
			AllocatableCPU:    allocatableCPU,
			AllocatableMemory: allocatableMemory,
			RequestedCPU:      0,
			RequestedMemory:   0,
			Pods:              make([]PodInfo, 0),
		}

		for _, pod := range podsOnNode {
			podKey := PodKey{
				Namespace: pod.Namespace,
				PodName:   pod.Name,
			}

			currentCPU := getCurrentPodCPU(pod)
			currentMemory := getCurrentPodMemory(pod)
			limitCPU := getCurrentPodCPULimit(pod)
			limitMemory := getCurrentPodMemoryLimit(pod)

			nodeInfo.RequestedCPU += currentCPU
			nodeInfo.RequestedMemory += currentMemory

			podInfo := PodInfo{
				Namespace:       pod.Namespace,
				Name:            pod.Name,
				RequestedCPU:    currentCPU,
				RequestedMemory: currentMemory,
				LimitCPU:        limitCPU,
				LimitMemory:     limitMemory,
			}

			if workloadInfo, hasWorkloadInfo := podToWorkloadMap[podKey]; hasWorkloadInfo {
				podInfo.WorkloadKind = workloadInfo.Kind
				podInfo.WorkloadName = workloadInfo.Name
			}

			if stat, hasStat := podStats[podKey]; hasStat {
				podInfo.Stats = stat
			}

			for _, container := range pod.Spec.Containers {
				podInfo.ContainerResources = append(podInfo.ContainerResources, getCurrentContainerResources(&container))
			}

			for _, container := range pod.Spec.InitContainers {
				if IsSidecarContainer(container) {
					podInfo.ContainerResources = append(podInfo.ContainerResources, getCurrentContainerResources(&container))
				}
			}

			nodeInfo.Pods = append(nodeInfo.Pods, podInfo)
		}

		nodeMap[nodeName] = nodeInfo
	}

	logging.Infof(ctx, "Successfully created node stats mapping")
	return nodeMap, nil
}

func getCurrentPodMemory(pod *corev1.Pod) float64 {
	sumAppSidecar := 0.0
	maxInit := 0.0

	for _, c := range pod.Spec.Containers {
		if c.Resources.Requests == nil {
			continue
		}

		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			sumAppSidecar += float64(q.Value()) / BytesToMBDivisor
		}
	}

	for _, c := range pod.Spec.InitContainers {
		if c.Resources.Requests == nil {
			continue
		}

		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			memory := float64(q.Value()) / BytesToMBDivisor
			if IsSidecarContainer(c) {
				sumAppSidecar += memory
			} else {
				maxInit = max(maxInit, memory)
			}
		}
	}

	return max(sumAppSidecar, maxInit)
}

func getCurrentPodCPULimit(pod *corev1.Pod) float64 {
	var totalCPU float64
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits != nil {
			if cpuLimit, exists := container.Resources.Limits[corev1.ResourceCPU]; exists {
				totalCPU += float64(cpuLimit.MilliValue()) / 1000.0
			}
		}
	}
	return totalCPU
}

func getCurrentPodMemoryLimit(pod *corev1.Pod) float64 {
	var totalMemory float64
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits != nil {
			if memLimit, exists := container.Resources.Limits[corev1.ResourceMemory]; exists {
				totalMemory += float64(memLimit.Value()) / BytesToMBDivisor
			}
		}
	}
	return totalMemory
}

func getCurrentContainerResources(container *corev1.Container) *ContainerResources {
	var cpuRequest float64
	var cpuLimit float64
	var memoryRequest float64
	var memoryLimit float64

	if container.Resources.Requests != nil {
		if cpuRequestQuantity, exists := container.Resources.Requests[corev1.ResourceCPU]; exists {
			cpuRequest = float64(cpuRequestQuantity.MilliValue()) / 1000.0
		}
		if memoryRequestQuantity, exists := container.Resources.Requests[corev1.ResourceMemory]; exists {
			memoryRequest = float64(memoryRequestQuantity.Value()) / BytesToMBDivisor
		}
	}
	if container.Resources.Limits != nil {
		if cpuLimitQuantity, exists := container.Resources.Limits[corev1.ResourceCPU]; exists {
			cpuLimit = float64(cpuLimitQuantity.MilliValue()) / 1000.0
		}
		if memoryLimitQuantity, exists := container.Resources.Limits[corev1.ResourceMemory]; exists {
			memoryLimit = float64(memoryLimitQuantity.Value()) / BytesToMBDivisor
		}
	}
	return &ContainerResources{
		Name:          container.Name,
		CPURequest:    cpuRequest,
		CPULimit:      cpuLimit,
		MemoryRequest: memoryRequest,
		MemoryLimit:   memoryLimit,
	}
}

func getCurrentPodCPU(pod *corev1.Pod) float64 {
	sumAppSidecar := 0.0
	maxInit := 0.0

	for _, c := range pod.Spec.Containers {
		if c.Resources.Requests == nil {
			continue
		}

		if q, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			sumAppSidecar += float64(q.MilliValue()) / 1000.0
		}
	}

	for _, c := range pod.Spec.InitContainers {
		if c.Resources.Requests == nil {
			continue
		}

		if q, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			v := float64(q.MilliValue()) / 1000.0
			if IsSidecarContainer(c) {
				sumAppSidecar += v
			} else {
				maxInit = max(maxInit, v)
			}
		}
	}

	return max(sumAppSidecar, maxInit)
}

// BuildPodInfoFromPod builds PodInfo from a pod and optional workload info and stat.
// Used by the admission webhook to build PodInfo for the incoming pod.
func BuildPodInfoFromPod(pod *corev1.Pod, workloadInfo *WorkloadInfo, stat *WorkloadStat) PodInfo {
	info := PodInfo{
		Namespace:       pod.Namespace,
		Name:            pod.Name,
		RequestedCPU:    getCurrentPodCPU(pod),
		RequestedMemory: getCurrentPodMemory(pod),
		LimitCPU:        getCurrentPodCPULimit(pod),
		LimitMemory:     getCurrentPodMemoryLimit(pod),
	}
	if workloadInfo != nil {
		info.WorkloadKind = workloadInfo.Kind
		info.WorkloadName = workloadInfo.Name
	}
	if stat != nil {
		info.Stats = stat
	}
	for i := range pod.Spec.Containers {
		info.ContainerResources = append(info.ContainerResources, getCurrentContainerResources(&pod.Spec.Containers[i]))
	}
	for i := range pod.Spec.InitContainers {
		if IsSidecarContainer(pod.Spec.InitContainers[i]) {
			info.ContainerResources = append(info.ContainerResources, getCurrentContainerResources(&pod.Spec.InitContainers[i]))
		}
	}
	return info
}
