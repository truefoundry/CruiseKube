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

func TestComputeRecommendedResourceValuesKeepsMemoryLimitAtLeastTwiceRequest(t *testing.T) {
	tests := []struct {
		name           string
		recMemory      float64
		memMax7Day     float64
		oomMemory      float64
		wantRequest    float64
		wantExactLimit float64 // if non-zero, limit must equal this
	}{
		{
			// Original bug scenario: rec.Memory=1014 (from strategy with distributed
			// headroom), but 7-day max is only 255. Without the 2x-request floor,
			// limit would be max(1014, 510) = 1014 — barely above request.
			// With the fix: limit = max(1014*2, 510) = 2028.
			name:           "high_request_low_7day_max",
			recMemory:      1014,
			memMax7Day:     255,
			oomMemory:      0,
			wantRequest:    1014,
			wantExactLimit: 2028,
		},
		{
			// When 7-day max is large, the stats-derived limit dominates.
			// limit = max(500*2, 800*2) = max(1000, 1600) = 1600.
			name:           "large_7day_max_dominates",
			recMemory:      500,
			memMax7Day:     800,
			oomMemory:      0,
			wantRequest:    500,
			wantExactLimit: 1600,
		},
		{
			// OOM memory drives the limit when it's the largest signal.
			// limit = max(400*2, max(300, 600)*2) = max(800, 1200) = 1200.
			name:           "oom_drives_limit",
			recMemory:      400,
			memMax7Day:     300,
			oomMemory:      600,
			wantRequest:    400,
			wantExactLimit: 1200,
		},
		{
			// Small request: 2x-request floor ensures reasonable headroom.
			// limit = max(50*2, max(30, 0)*2) = max(100, 60) = 100.
			// But EnforceMinimumMemory(60) = 60 (>16), so limit = max(100, 60) = 100.
			name:           "small_request_2x_floor",
			recMemory:      50,
			memMax7Day:     30,
			oomMemory:      0,
			wantRequest:    50,
			wantExactLimit: 100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := PodContainerRecommendation{
				ContainerName: "main",
				CPU:           1,
				Memory:        tc.recMemory,
				PodInfo: PodInfo{
					Stats: &WorkloadStat{
						ContainerStats: []ContainerStats{
							{
								ContainerName: "main",
								MemoryStats: &MemoryStats{
									OOMMemory: tc.oomMemory,
								},
								Memory7Day: &Memory7DayStats{
									Max: tc.memMax7Day,
								},
							},
						},
					},
				},
			}

			_, memoryRequest, _, memoryLimit := ComputeRecommendedResourceValues(context.Background(), rec, 4)

			if memoryRequest != tc.wantRequest {
				t.Fatalf("memory request: got %v, want %v", memoryRequest, tc.wantRequest)
			}
			if memoryLimit < memoryRequest {
				t.Fatalf("memory limit (%v) < memory request (%v)", memoryLimit, memoryRequest)
			}
			if memoryLimit < memoryRequest*2 {
				t.Fatalf("memory limit (%v) < 2x memory request (%v)", memoryLimit, memoryRequest*2)
			}
			if tc.wantExactLimit > 0 && memoryLimit != tc.wantExactLimit {
				t.Fatalf("memory limit: got %v, want %v", memoryLimit, tc.wantExactLimit)
			}
		})
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
