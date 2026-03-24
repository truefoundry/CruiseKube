package applystrategies

import (
	"context"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestOptimizeNodeHandlesMissingSimplePredictionsDuringEviction(t *testing.T) {
	strategy := NewAdjustAmongstPodsDistributedStrategy(context.Background())

	result, err := strategy.OptimizeNode(context.Background(), nil, map[string]*types.WorkloadOverrideInfo{}, utils.NodeOptimizationData{
		NodeName:          "node-1",
		AllocatableCPU:    0,
		AllocatableMemory: 0,
		PodInfos: []utils.PodInfo{
			{
				Namespace:       "default",
				Name:            "pod-1",
				WorkloadKind:    "Deployment",
				RequestedCPU:    2,
				RequestedMemory: 512,
				ContainerResources: []*utils.ContainerResources{
					{Name: "app", CPURequest: 2, MemoryRequest: 512},
				},
				Stats: &utils.WorkloadStat{
					WorkloadIdentifier: "Deployment:default:workload-1",
					OriginalContainerResources: []utils.OriginalContainerResources{
						{Name: "app", Type: types.AppContainer, CPURequest: 2, MemoryRequest: 512},
					},
					ContainerStats: []utils.ContainerStats{
						{
							ContainerName: "app",
							ContainerType: types.AppContainer,
							CPUStats:      &utils.CPUStats{P75: 1},
							MemoryStats:   &utils.MemoryStats{P75: 256},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.PodContainerRecommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result.PodContainerRecommendations))
	}

	rec := result.PodContainerRecommendations[0]
	if !rec.Evict {
		t.Fatalf("expected eviction recommendation")
	}
	if rec.CPU != 1 {
		t.Fatalf("expected cpu fallback to P75, got %v", rec.CPU)
	}
	if rec.Memory != 256 {
		t.Fatalf("expected memory fallback to P75, got %v", rec.Memory)
	}
}
