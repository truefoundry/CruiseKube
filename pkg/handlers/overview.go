package handlers

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ratioLookbackDays = 7
)

func percent(part, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return part / total * 100
}

func (deps HandlerDependencies) getClusterNodeCount(ctx context.Context, clusterID string) int {
	if deps.ClusterManager == nil {
		return 0
	}
	clients, err := deps.ClusterManager.GetClusterClients(clusterID)
	if err != nil || clients == nil || clients.KubeClient == nil {
		return 0
	}
	nodes, err := clients.KubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Warnf(ctx, "Failed to list nodes for cluster %s: %v", clusterID, err)
		return 0
	}
	return len(nodes.Items)
}

// clusterResourcesFromDB holds cluster resource values and the 7-day average request/allocatable ratios.
type clusterResourcesFromDB struct {
	Resources         types.ClusterResourcesDTO
	RecommendedCPU    float64
	RecommendedMemory float64
	OriginalCPU       float64
	OriginalMemory    float64
	ReqAllocRatioCPU  float64
	ReqAllocRatioMem  float64
}

// getClusterResourcesFromDatabase loads cluster allocatable/requested/utilised from the snapshots table,
// and computes request-to-allocatable ratios as the average over the last 7 days using at most 10 samples (one per day).
func (deps HandlerDependencies) getClusterResourcesFromDatabase(ctx context.Context, clusterID string) clusterResourcesFromDB {
	out := clusterResourcesFromDB{
		Resources: types.ClusterResourcesDTO{
			CPU:    types.ClusterResourceDTO{Utilised: 0, Requested: 0, Allocatable: 0},
			Memory: types.ClusterResourceDTO{Utilised: 0, Requested: 0, Allocatable: 0},
		},
		RecommendedCPU:    0,
		RecommendedMemory: 0,
		OriginalCPU:       0,
		OriginalMemory:    0,
		ReqAllocRatioCPU:  1.0,
		ReqAllocRatioMem:  1.0,
	}
	if deps.Storage == nil {
		return out
	}

	endTime := time.Now().UTC()
	startTime := endTime.AddDate(0, 0, -ratioLookbackDays)
	snapshots, err := deps.Storage.GetSnapshotsInRange(clusterID, startTime, endTime)
	if err != nil {
		logging.Warnf(ctx, "Failed to get snapshots for cluster %s: %v", clusterID, err)
		return out
	}
	if len(snapshots) == 0 {
		return out
	}

	// Use the most recent snapshot for current cluster state.
	latest := snapshots[len(snapshots)-1]
	out.Resources.CPU.Allocatable = latest.Data.CPU.CurrentAllocatable
	out.Resources.CPU.Requested = latest.Data.CPU.CurrentRequested
	out.Resources.CPU.Utilised = latest.Data.CPU.CurrentUtilized
	out.Resources.Memory.Allocatable = latest.Data.Memory.CurrentAllocatable
	out.Resources.Memory.Requested = latest.Data.Memory.CurrentRequested
	out.Resources.Memory.Utilised = latest.Data.Memory.CurrentUtilized
	out.RecommendedCPU = latest.Data.CPU.RecommendedRequested
	out.RecommendedMemory = latest.Data.Memory.RecommendedRequested
	out.OriginalCPU = latest.Data.CPU.WorkloadRequested
	out.OriginalMemory = latest.Data.Memory.WorkloadRequested

	if latest.Data.CPU.CurrentAllocatable == 0 || latest.Data.Memory.CurrentAllocatable == 0 {
		return out
	}
	out.ReqAllocRatioCPU = latest.Data.CPU.CurrentRequested / latest.Data.CPU.CurrentAllocatable
	out.ReqAllocRatioMem = latest.Data.Memory.CurrentRequested / latest.Data.Memory.CurrentAllocatable

	return out
}

func (deps HandlerDependencies) OverviewHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	details, recAgg, clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem, err := deps.getWorkloadsData(ctx, clusterID)
	_ = recAgg
	_ = clusterReqCPU
	_ = clusterReqMem
	_ = clusterRecCPU
	_ = clusterRecMem
	if err != nil {
		logging.Errorf(ctx, "Failed to get workloads for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	p := deps.getEffectivePricing(ctx, clusterID)
	dbRes := deps.getClusterResourcesFromDatabase(ctx, clusterID)
	clusterRes := dbRes.Resources
	reqAllocRatioCPU := dbRes.ReqAllocRatioCPU
	reqAllocRatioMem := dbRes.ReqAllocRatioMem

	// Cost and savings mirror the summary API: current infra cost, current savings vs workload request, and possible savings vs recommendation.
	currentCostDollars := (clusterRes.CPU.Allocatable*p.CPUPerCorePerHour + clusterRes.Memory.Allocatable*p.MemPerGBPerHour) * defaultHoursPerMonth
	workloadCostDollars := (dbRes.OriginalCPU/reqAllocRatioCPU)*p.CPUPerCorePerHour*defaultHoursPerMonth + (dbRes.OriginalMemory/reqAllocRatioMem)*p.MemPerGBPerHour*defaultHoursPerMonth
	optimizedCostDollars := (dbRes.RecommendedCPU/reqAllocRatioCPU)*p.CPUPerCorePerHour*defaultHoursPerMonth + (dbRes.RecommendedMemory/reqAllocRatioMem)*p.MemPerGBPerHour*defaultHoursPerMonth

	currentCostDollars = math.Round(currentCostDollars)
	workloadCostDollars = math.Round(workloadCostDollars)
	optimizedCostDollars = math.Round(optimizedCostDollars)

	// Cluster utilization is the highest of CPU and memory allocatable utilization (only across available dimensions).
	var cpuRatio, memRatio *float64
	if clusterRes.CPU.Allocatable > 0 {
		r := clusterRes.CPU.Utilised / clusterRes.CPU.Allocatable
		cpuRatio = &r
	}
	if clusterRes.Memory.Allocatable > 0 {
		r := clusterRes.Memory.Utilised / clusterRes.Memory.Allocatable
		memRatio = &r
	}
	clusterUtilisation := 0.0
	switch {
	case cpuRatio != nil && memRatio != nil:
		clusterUtilisation = min(*cpuRatio, *memRatio) * 100
	case cpuRatio != nil:
		clusterUtilisation = *cpuRatio * 100
	case memRatio != nil:
		clusterUtilisation = *memRatio * 100
	}

	// Adoption coverage is workload-count based, while CPU/memory coverage is weighted by current requested resources.
	totalWorkloads := len(details)
	totalRequestedCPU := 0.0
	totalRequestedMem := 0.0
	enabledRequestedCPU := 0.0
	enabledRequestedMem := 0.0
	optimizableWorkloads := 0
	nonOptimizableWorkloads := 0
	optimizableButExcludedWorkloads := 0

	for i := range details {
		d := details[i]
		totalRequestedCPU += d.CPU.CurrentPerPod * float64(d.PodsCount)
		totalRequestedMem += d.Memory.CurrentPerPod * float64(d.PodsCount)
		switch {
		case isNonOptimizableWorkload(d):
			totalRequestedCPU -= d.CPU.CurrentPerPod * float64(d.PodsCount)
			totalRequestedMem -= d.Memory.CurrentPerPod * float64(d.PodsCount)
			nonOptimizableWorkloads++
		case d.Config.CruiseEnabled:
			optimizableWorkloads++
			enabledRequestedCPU += d.CPU.CurrentPerPod * float64(d.PodsCount)
			enabledRequestedMem += d.Memory.CurrentPerPod * float64(d.PodsCount)
		default:
			optimizableButExcludedWorkloads++
		}
	}

	enabledCPUCoverage := percent(enabledRequestedCPU, totalRequestedCPU)
	disabledCPUCoverage := percent(totalRequestedCPU-enabledRequestedCPU, totalRequestedCPU)
	enabledMemoryCoverage := percent(enabledRequestedMem, totalRequestedMem)
	disabledMemoryCoverage := percent(totalRequestedMem-enabledRequestedMem, totalRequestedMem)

	c.JSON(http.StatusOK, types.OverviewResponse{
		CurrentMonthlyCost: int(currentCostDollars),
		CurrentSavings:     int(workloadCostDollars - currentCostDollars),
		PossibleSavings:    int(workloadCostDollars - optimizedCostDollars),
		ClusterUtilisation: math.Round(clusterUtilisation),
		NodeCount:          deps.getClusterNodeCount(ctx, clusterID),
		Coverage: types.OverviewCoverage{
			Adoption: types.OverviewCoverageBreakdown{
				Optimizable:            optimizableWorkloads,
				NonOptimizable:         nonOptimizableWorkloads,
				OptimizableButExcluded: optimizableButExcludedWorkloads,
				Total:                  totalWorkloads,
			},
			CPUCoverage: types.OverviewCoverageBreakdownTypo{
				Enabed:   enabledCPUCoverage,
				Disabled: disabledCPUCoverage,
			},
			MemoryCoverage: types.OverviewCoverageBreakdownTypo{
				Enabed:   enabledMemoryCoverage,
				Disabled: disabledMemoryCoverage,
			},
		},
		CPUStats: types.OverviewResourceStats{
			Allocatable:       clusterRes.CPU.Allocatable,
			Requested:         clusterRes.CPU.Requested,
			WorkloadRequested: dbRes.OriginalCPU,
			Usage:             clusterRes.CPU.Utilised,
			Recommended:       dbRes.RecommendedCPU,
		},
		MemoryStats: types.OverviewResourceStats{
			Allocatable:       clusterRes.Memory.Allocatable,
			Requested:         clusterRes.Memory.Requested,
			WorkloadRequested: dbRes.OriginalMemory,
			Usage:             clusterRes.Memory.Utilised,
			Recommended:       dbRes.RecommendedMemory,
		},
	})
}

func isNonOptimizableWorkload(d types.WorkloadDetail) bool {
	if d.Constraints.IsGPUWorkload || d.Config.HPAEnabled {
		return true
	}
	return len(d.Config.ExcludedCodes) > 0
}
