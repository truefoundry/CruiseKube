package utils

import (
	"fmt"
	"strings"
)

type NodeResourceInfo struct {
	AllocatableCPU    float64   `json:"allocatable_cpu"`
	AllocatableMemory float64   `json:"allocatable_memory"`
	RequestedCPU      float64   `json:"requested_cpu"`
	RequestedMemory   float64   `json:"requested_memory"`
	Pods              []PodInfo `json:"pods"`

	NodeType          string `json:"node_type"`
	EventReason       string `json:"event_reason"`
	EventMessage      string `json:"event_message"`
	KarpenterNodePool string `json:"karpenter_node_pool"`
}

type ContainerResources struct {
	Name          string  `json:"name"`
	CPURequest    float64 `json:"cpu_request"`
	CPULimit      float64 `json:"cpu_limit"`
	MemoryRequest float64 `json:"memory_request,omitempty"`
	MemoryLimit   float64 `json:"memory_limit,omitempty"`
}

type PodInfo struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	WorkloadKind string `json:"workload_kind,omitempty"`
	WorkloadName string `json:"workload_name,omitempty"`
	// Sum of cpu request for all containers in the pod
	RequestedCPU float64 `json:"requested_cpu"`
	// Sum of memory request for all containers in the pod
	RequestedMemory    float64               `json:"requested_memory"`
	LimitCPU           float64               `json:"limit_cpu"`
	LimitMemory        float64               `json:"limit_memory"`
	Stats              *WorkloadStat         `json:"stats,omitempty"`
	ContainerResources []*ContainerResources `json:"container_resources,omitempty"`
}

func (p *PodInfo) IsBestEffortPod() bool {
	for _, container := range p.ContainerResources {
		if container.CPURequest != 0 || container.CPULimit != 0 || container.MemoryRequest != 0 || container.MemoryLimit != 0 {
			return false
		}
	}
	return true
}

func (p *PodInfo) IsGuaranteedPod() bool {
	for _, container := range p.ContainerResources {
		if container.CPURequest > 0 && container.CPULimit != container.CPURequest {
			return false
		}
		if container.MemoryRequest > 0 && container.MemoryLimit != container.MemoryRequest {
			return false
		}
	}
	return true
}

func (p *PodInfo) GetContainerResource(containerName string) (*ContainerResources, error) {
	for _, containerResource := range p.ContainerResources {
		if strings.EqualFold(containerResource.Name, containerName) {
			return containerResource, nil
		}
	}
	return nil, fmt.Errorf("container resource %s not found", containerName)
}

func (p *PodInfo) GetOriginalContainerResource(containerName string) (*ContainerResources, error) {
	// p.Stats can be nil if the pod has no stats generated yet.
	if p.Stats == nil || p.Stats.OriginalContainerResources == nil || len(p.Stats.OriginalContainerResources) == 0 {
		return nil, fmt.Errorf("stats not available")
	}
	for _, containerResource := range p.Stats.OriginalContainerResources {
		if strings.EqualFold(containerResource.Name, containerName) {
			return &ContainerResources{
				Name:          containerResource.Name,
				CPURequest:    containerResource.CPURequest,
				CPULimit:      containerResource.CPULimit,
				MemoryRequest: containerResource.MemoryRequest,
				MemoryLimit:   containerResource.MemoryLimit,
			}, nil
		}
	}
	return nil, fmt.Errorf("container resource %s not found in stats", containerName)
}
