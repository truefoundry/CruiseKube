package utils

import (
	"context"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestShouldGenerateRecommendationRejectsIncompleteStats(t *testing.T) {
	podInfo := &PodInfo{
		Namespace: "ns",
		Name:      "pod-1",
		Stats: &WorkloadStat{
			WorkloadIdentifier: "Deployment:ns:app",
			CreationTime:       time.Now().Add(-48 * time.Hour),
			Metadata: &types.WorkloadStatMetadata{
				Incomplete: true,
			},
		},
		ContainerResources: []*ContainerResources{
			{
				Name:          "main",
				CPURequest:    1,
				CPULimit:      1,
				MemoryRequest: 256,
				MemoryLimit:   256,
			},
		},
	}

	ok, reason := ShouldGenerateRecommendation(context.Background(), podInfo, ApplyCheckInput{
		NewWorkloadThresholdHours: 1,
	})

	if ok {
		t.Fatal("expected incomplete workload stats to be rejected")
	}
	if reason != "workload has incomplete stats" {
		t.Fatalf("expected incomplete-stats reason, got %q", reason)
	}
}

func TestComputeRecommendedResourceValuesKeepsMemoryLimitAtLeastRequest(t *testing.T) {
	rec := PodContainerRecommendation{
		ContainerName: "main",
		CPU:           1,
		Memory:        1014,
		PodInfo: PodInfo{
			Stats: &WorkloadStat{
				ContainerStats: []ContainerStats{
					{
						ContainerName: "main",
						MemoryStats: &MemoryStats{
							OOMMemory: 0,
						},
						Memory7Day: &Memory7DayStats{
							Max: 255,
						},
					},
				},
			},
		},
	}

	_, memoryRequest, _, memoryLimit := ComputeRecommendedResourceValues(context.Background(), rec, 4)
	if memoryLimit < memoryRequest {
		t.Fatalf("expected memory limit >= memory request, got limit=%v request=%v", memoryLimit, memoryRequest)
	}
	if memoryRequest != 1014 {
		t.Fatalf("expected memory request to be 1014MB, got %v", memoryRequest)
	}
	if memoryLimit != 1014 {
		t.Fatalf("expected memory limit to be clamped to request (1014MB), got %v", memoryLimit)
	}
}

func TestComputeRecommendedResourceValuesKeepsCPULimitAtLeastRequestWhenSet(t *testing.T) {
	rec := PodContainerRecommendation{
		ContainerName: "main",
		CPU:           2.5,
		Memory:        256,
	}

	cpuRequest, _, cpuLimit, _ := ComputeRecommendedResourceValues(context.Background(), rec, 1.0)
	if cpuRequest != 2.5 {
		t.Fatalf("expected cpu request to be 2.5, got %v", cpuRequest)
	}
	if cpuLimit < cpuRequest {
		t.Fatalf("expected cpu limit >= cpu request, got limit=%v request=%v", cpuLimit, cpuRequest)
	}
	if cpuLimit != 2.5 {
		t.Fatalf("expected cpu limit to be clamped to request (2.5), got %v", cpuLimit)
	}
}
