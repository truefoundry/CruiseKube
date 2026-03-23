package task

import (
	"testing"

	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestHasCompleteSimplePredictions(t *testing.T) {
	workloadStat := &utils.WorkloadStat{
		ContainerStats: []utils.ContainerStats{
			{
				ContainerName:           "app",
				ContainerType:           types.AppContainer,
				SimplePredictionsCPU:    &types.SimplePrediction{MaxValue: 1},
				SimplePredictionsMemory: &types.SimplePrediction{MaxValue: 512},
			},
			{
				ContainerName:           "sidecar",
				ContainerType:           types.SidecarContainer,
				SimplePredictionsCPU:    &types.SimplePrediction{MaxValue: 0.2},
				SimplePredictionsMemory: &types.SimplePrediction{MaxValue: 128},
			},
		},
	}

	containerResources := []utils.OriginalContainerResources{
		{Name: "app", Type: types.AppContainer},
		{Name: "sidecar", Type: types.SidecarContainer},
		{Name: "init", Type: types.InitContainer},
	}

	if !hasCompleteSimplePredictions(workloadStat, containerResources) {
		t.Fatalf("expected workload stat with predictions on all non-init containers to be accepted")
	}
}

func TestHasCompleteSimplePredictionsRejectsMissingPredictions(t *testing.T) {
	workloadStat := &utils.WorkloadStat{
		ContainerStats: []utils.ContainerStats{
			{
				ContainerName:           "app",
				ContainerType:           types.AppContainer,
				SimplePredictionsCPU:    &types.SimplePrediction{MaxValue: 1},
				SimplePredictionsMemory: &types.SimplePrediction{MaxValue: 512},
			},
			{
				ContainerName:        "sidecar",
				ContainerType:        types.SidecarContainer,
				SimplePredictionsCPU: &types.SimplePrediction{MaxValue: 0.2},
			},
		},
	}

	containerResources := []utils.OriginalContainerResources{
		{Name: "app", Type: types.AppContainer},
		{Name: "sidecar", Type: types.SidecarContainer},
	}

	if hasCompleteSimplePredictions(workloadStat, containerResources) {
		t.Fatalf("expected workload stat missing simple memory prediction to be rejected")
	}
}

func TestHasCompleteSimplePredictionsRejectsMissingContainerStats(t *testing.T) {
	workloadStat := &utils.WorkloadStat{
		ContainerStats: []utils.ContainerStats{
			{
				ContainerName:           "app",
				ContainerType:           types.AppContainer,
				SimplePredictionsCPU:    &types.SimplePrediction{MaxValue: 1},
				SimplePredictionsMemory: &types.SimplePrediction{MaxValue: 512},
			},
		},
	}

	containerResources := []utils.OriginalContainerResources{
		{Name: "app", Type: types.AppContainer},
		{Name: "sidecar", Type: types.SidecarContainer},
	}

	if hasCompleteSimplePredictions(workloadStat, containerResources) {
		t.Fatalf("expected workload stat missing a non-init container to be rejected")
	}
}
