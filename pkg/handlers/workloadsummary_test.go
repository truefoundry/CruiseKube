package handlers

import (
	"math"
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

func TestAggregatePodCurrentsForWorkloadWithoutRowsReturnsZeroValues(t *testing.T) {
	stat := &types.WorkloadStat{Replicas: 0}

	current := aggregatePodCurrentsForWorkload(nil, stat)

	if math.IsNaN(current.CurrentCPURequest) || math.IsNaN(current.CurrentMemoryRequest) {
		t.Fatalf("expected finite current requests, got cpu=%v mem=%v", current.CurrentCPURequest, current.CurrentMemoryRequest)
	}
	if current.CurrentCPURequest != 0 || current.CurrentMemoryRequest != 0 {
		t.Fatalf("expected zero current requests, got cpu=%v mem=%v", current.CurrentCPURequest, current.CurrentMemoryRequest)
	}
}

func TestFillWorkloadDetailDollarsScalesWithPodsCount(t *testing.T) {
	pricing := workloadPricing{CPUPerCorePerHour: 0.05, MemPerGBPerHour: 0.01}
	agg := workloadRecAgg{TotalCPU: 0, TotalMem: 0}

	dA := &types.WorkloadDetail{PodsCount: 2, CPU: types.WorkloadCPU{Current: 1.0}, Memory: types.WorkloadMemory{Current: 512}}
	dB := &types.WorkloadDetail{PodsCount: 5, CPU: types.WorkloadCPU{Current: 1.0}, Memory: types.WorkloadMemory{Current: 512}}

	fillWorkloadDetailDollars(dA, agg, pricing)
	fillWorkloadDetailDollars(dB, agg, pricing)

	if dA.DollarSavingsPerMonth <= 0 {
		t.Fatalf("expected positive savings for dA, got %v", dA.DollarSavingsPerMonth)
	}
	ratio := dB.DollarSavingsPerMonth / dA.DollarSavingsPerMonth
	expectedRatio := float64(5) / float64(2)
	if math.Abs(ratio-expectedRatio) > 0.001 {
		t.Fatalf("expected savings ratio of %.1f, got %.4f (dA=%.4f, dB=%.4f)", expectedRatio, ratio, dA.DollarSavingsPerMonth, dB.DollarSavingsPerMonth)
	}
}
