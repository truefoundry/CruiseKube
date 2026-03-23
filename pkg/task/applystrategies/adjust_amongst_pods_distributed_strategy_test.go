package applystrategies

import (
	"context"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestOptimizeNodeKeepsPodWhenEvictionPredictionsAreMissing(t *testing.T) {
	strategy := NewAdjustAmongstPodsDistributedStrategy(context.Background())

	pod := utils.PodInfo{
		Namespace:    "ns",
		Name:         "pod-a",
		WorkloadKind: utils.DeploymentKind,
		WorkloadName: "workload-a",
		Stats: &types.WorkloadStat{
			WorkloadIdentifier: "Deployment:ns:workload-a",
			Kind:               utils.DeploymentKind,
			Namespace:          "ns",
			Name:               "workload-a",
			CreationTime:       time.Now().Add(-2 * time.Hour),
			EvictionRanking:    types.EvictionRankingHigh,
			ContainerStats: []types.ContainerStats{
				{
					ContainerName: "app",
					ContainerType: types.AppContainer,
					CPUStats:      &types.CPUStats{P75: 0.5},
					MemoryStats:   &types.MemoryStats{P75: 256},
				},
			},
			OriginalContainerResources: []types.OriginalContainerResources{
				{
					Name:          "app",
					Type:          types.AppContainer,
					CPURequest:    1,
					CPULimit:      1,
					MemoryRequest: 512,
					MemoryLimit:   512,
				},
			},
		},
		ContainerResources: []*utils.ContainerResources{
			{
				Name:          "app",
				CPURequest:    1,
				CPULimit:      1,
				MemoryRequest: 512,
				MemoryLimit:   512,
			},
		},
	}

	result, err := strategy.OptimizeNode(context.Background(), nil, nil, utils.NodeOptimizationData{
		NodeName:          "node-a",
		AllocatableCPU:    0.1,
		AllocatableMemory: 128,
		PodInfos:          []utils.PodInfo{pod},
	})
	if err != nil {
		t.Fatalf("OptimizeNode returned error: %v", err)
	}

	if len(result.PodContainerRecommendations) != 1 {
		t.Fatalf("expected 1 non-eviction recommendation, got %d", len(result.PodContainerRecommendations))
	}

	rec := result.PodContainerRecommendations[0]
	if rec.Evict {
		t.Fatalf("expected pod to remain non-evicted when predictions are missing")
	}
	if rec.PodInfo.Name != pod.Name || rec.PodInfo.Namespace != pod.Namespace || rec.ContainerName != "app" {
		t.Fatalf("unexpected recommendation target: %#v", rec)
	}
	if rec.CPU != 0.5 {
		t.Fatalf("expected CPU recommendation to fall back to p75, got %v", rec.CPU)
	}
	if rec.Memory != 256 {
		t.Fatalf("expected memory recommendation to fall back to p75, got %v", rec.Memory)
	}
}
