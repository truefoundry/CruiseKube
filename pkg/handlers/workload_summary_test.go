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
}

func TestIsNonOptimizableWorkloadTreatsIncompleteStatsAsNonOptimizable(t *testing.T) {
	workload := types.WorkloadDetail{
		Config: types.WorkloadConfig{
			ExcludedCodes: []types.ExcludedCode{types.ExcludedCodeIncomplete},
		},
	}

	if !isNonOptimizableWorkload(workload) {
		t.Fatal("expected incomplete workload to be classified as non-optimizable")
	}
}
