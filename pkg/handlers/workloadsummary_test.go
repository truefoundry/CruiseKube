package handlers

import (
	"testing"

	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestAggregateRecsForWorkloadWithoutRowsUsesReplicaTotals(t *testing.T) {
	stat := &types.WorkloadStat{
		Replicas: 3,
		OriginalContainerResources: []types.OriginalContainerResources{
			{Name: "main", Type: types.AppContainer, CPURequest: 1.5, MemoryRequest: 512},
		},
	}

	agg := aggregateRecsForWorkload(nil, stat)

	if agg.CPUAvg != 1.5 || agg.MemAvg != 512 {
		t.Fatalf("expected per-pod recommendation to match current request, got cpu=%v mem=%v", agg.CPUAvg, agg.MemAvg)
	}
	if agg.TotalCPU != 4.5 || agg.TotalMem != 1536 {
		t.Fatalf("expected totals to include replicas, got cpu=%v mem=%v", agg.TotalCPU, agg.TotalMem)
	}
	if agg.HasActual {
		t.Fatal("expected HasActual false when there are no recommendation rows")
	}
}

func TestAggregateRecsForWorkloadUsesStoredActualResources(t *testing.T) {
	stat := &types.WorkloadStat{
		Replicas: 2,
		OriginalContainerResources: []types.OriginalContainerResources{
			{Name: "main", Type: types.AppContainer, CPURequest: 2.0, MemoryRequest: 1024},
		},
	}
	recs := []parsedPodRecommendation{
		{WorkloadID: "Deployment:ns:app", Namespace: "ns", Pod: "app-1", Rec: types.PodResourceRecommendation{
			CPURequest: 1.0, MemoryRequest: 512, WorkloadPodCount: 2,
			CurrentCPURequest: 2.0, CurrentMemoryRequest: 1024,
		}},
		{WorkloadID: "Deployment:ns:app", Namespace: "ns", Pod: "app-2", Rec: types.PodResourceRecommendation{
			CPURequest: 1.0, MemoryRequest: 512, WorkloadPodCount: 2,
			CurrentCPURequest: 2.0, CurrentMemoryRequest: 1024,
		}},
	}
	agg := aggregateRecsForWorkload(recs, stat)
	if !agg.HasActual {
		t.Fatal("expected HasActual true when rows carry current_* / workload_pod_count")
	}
	if agg.ActualPodCount != 2 {
		t.Fatalf("ActualPodCount: want 2, got %d", agg.ActualPodCount)
	}
	if agg.ActualCPUAvg != 2.0 || agg.ActualMemAvg != 1024 {
		t.Fatalf("actual per-pod avg: want cpu=2 mem=1024, got cpu=%v mem=%v", agg.ActualCPUAvg, agg.ActualMemAvg)
	}
	if agg.ActualTotalCPU != 4.0 || agg.ActualTotalMem != 2048 {
		t.Fatalf("actual totals: want cpu=4 mem=2048, got cpu=%v mem=%v", agg.ActualTotalCPU, agg.ActualTotalMem)
	}
}

func TestIsNonOptimizableWorkloadTreatsAnyExcludedCodeAsNonOptimizable(t *testing.T) {
	workload := types.WorkloadDetail{
		Config: types.WorkloadConfig{
			ExcludedCodes: []types.ExcludedCode{types.ExcludedCodeCPUHPA},
		},
	}

	if !isNonOptimizableWorkload(workload) {
		t.Fatal("expected workload with excluded codes to be classified as non-optimizable")
	}
}
