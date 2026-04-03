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
