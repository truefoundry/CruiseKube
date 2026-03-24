package utils

import (
	"context"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestShouldGenerateRecommendationRejectsIncompleteContainerStats(t *testing.T) {
	pod := &PodInfo{
		Namespace: "default",
		Name:      "pod-1",
		Stats: &WorkloadStat{
			CreationTime: time.Now().Add(-2 * time.Hour),
			OriginalContainerResources: []OriginalContainerResources{
				{Name: "app", Type: types.AppContainer},
			},
			ContainerStats: []ContainerStats{
				{
					ContainerName: "app",
					ContainerType: types.AppContainer,
					CPUStats:      &CPUStats{P75: 1},
					MemoryStats:   &MemoryStats{P75: 256},
				},
			},
		},
	}

	ok, reason := ShouldGenerateRecommendation(context.Background(), pod, ApplyCheckInput{
		NewWorkloadThresholdHours: 1,
	})
	if ok {
		t.Fatalf("expected incomplete stats to be rejected")
	}
	if reason != "incomplete stats for container app" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}
