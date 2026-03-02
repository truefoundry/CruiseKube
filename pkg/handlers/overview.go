package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func percent(part, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return part / total * 100
}

func getClusterNodeCount(ctx *gin.Context, clusterID string) int {
	mgr, _ := ctx.Get("clusterManager")
	if mgr == nil {
		return 0
	}
	clients, err := mgr.(cluster.Manager).GetClusterClients(clusterID)
	if err != nil || clients == nil || clients.KubeClient == nil {
		return 0
	}
	nodes, err := clients.KubeClient.CoreV1().Nodes().List(ctx.Request.Context(), metav1.ListOptions{})
	if err != nil {
		logging.Warnf(ctx.Request.Context(), "Failed to list nodes for cluster %s: %v", clusterID, err)
		return 0
	}
	return len(nodes.Items)
}

func OverviewHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	details, _, clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem, err := getWorkloadsData(ctx, clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	p := getEffectivePricing(ctx, clusterID)
	clusterRes := getClusterResourcesFromPrometheus(ctx, c, clusterID)

	// Match summary API logic by normalizing requested/recommended resources with request-to-allocatable ratios.
	reqAllocRatioCPU := 1.0
	if clusterRes.CPU.Allocatable > 0 {
		reqAllocRatioCPU = clusterRes.CPU.Requested / clusterRes.CPU.Allocatable
	}
	reqAllocRatioMem := 1.0
	if clusterRes.Memory.Allocatable > 0 {
		reqAllocRatioMem = clusterRes.Memory.Requested / clusterRes.Memory.Allocatable
	}

	// Memory request/recommendation from workload stats are in MiB; convert to GiB to align with pricing units.
	requestedMemGB := clusterReqMem / 1024
	recommendedMemGB := clusterRecMem / 1024

	// Cost and savings mirror the summary API: current infra cost, current savings vs workload request, and possible savings vs recommendation.
	currentCostDollars := (clusterRes.CPU.Allocatable*p.CPUPerCorePerHour + clusterRes.Memory.Allocatable*p.MemPerGBPerHour) * defaultHoursPerMonth
	workloadCostDollars := (clusterReqCPU/reqAllocRatioCPU)*p.CPUPerCorePerHour*defaultHoursPerMonth + (requestedMemGB/reqAllocRatioMem)*p.MemPerGBPerHour*defaultHoursPerMonth
	optimizedCostDollars := (clusterRecCPU/reqAllocRatioCPU)*p.CPUPerCorePerHour*defaultHoursPerMonth + (recommendedMemGB/reqAllocRatioMem)*p.MemPerGBPerHour*defaultHoursPerMonth

	// Cluster utilization is the average of CPU and memory allocatable utilization percentages (only across available dimensions).
	utilRatioSum := 0.0
	utilRatioCount := 0.0
	if clusterRes.CPU.Allocatable > 0 {
		utilRatioSum += clusterRes.CPU.Utilised / clusterRes.CPU.Allocatable
		utilRatioCount++
	}
	if clusterRes.Memory.Allocatable > 0 {
		utilRatioSum += clusterRes.Memory.Utilised / clusterRes.Memory.Allocatable
		utilRatioCount++
	}
	clusterUtilistion := 0.0
	if utilRatioCount > 0 {
		clusterUtilistion = utilRatioSum / utilRatioCount * 100
	}

	// Adoption coverage is workload-count based, while CPU/memory coverage is weighted by current requested resources.
	totalWorkloads := float64(len(details))
	enabledWorkloads := 0.0
	totalRequestedCPU := 0.0
	totalRequestedMem := 0.0
	enabledRequestedCPU := 0.0
	enabledRequestedMem := 0.0

	for i := range details {
		d := details[i]
		totalRequestedCPU += d.CPU.Current
		totalRequestedMem += d.Memory.Current
		if d.Config.CruiseEnabled {
			enabledWorkloads++
			enabledRequestedCPU += d.CPU.Current
			enabledRequestedMem += d.Memory.Current
		}
	}

	enabledAdoption := percent(enabledWorkloads, totalWorkloads)
	disabledAdoption := percent(totalWorkloads-enabledWorkloads, totalWorkloads)
	enabledCPUCoverage := percent(enabledRequestedCPU, totalRequestedCPU)
	disabledCPUCoverage := percent(totalRequestedCPU-enabledRequestedCPU, totalRequestedCPU)
	enabledMemoryCoverage := percent(enabledRequestedMem, totalRequestedMem)
	disabledMemoryCoverage := percent(totalRequestedMem-enabledRequestedMem, totalRequestedMem)

	c.JSON(http.StatusOK, types.OverviewResponse{
		CurrentMonthlyCost: int(currentCostDollars),
		CurrentSavings:     int(workloadCostDollars - currentCostDollars),
		PossibleSavings:    int(workloadCostDollars - optimizedCostDollars),
		ClusterUtilistion:  clusterUtilistion,
		NodeCount:          getClusterNodeCount(c, clusterID),
		Coverage: types.OverviewCoverage{
			Adoption: types.OverviewCoverageBreakdown{
				Enabled:  enabledAdoption,
				Disabled: disabledAdoption,
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
			Allocatable: clusterRes.CPU.Allocatable,
			Requested:   clusterRes.CPU.Requested,
			Usage:       clusterRes.CPU.Utilised,
			Recommended: clusterRecCPU,
		},
		MemoryStats: types.OverviewResourceStats{
			Allocatable: clusterRes.Memory.Allocatable,
			Requested:   clusterRes.Memory.Requested,
			Usage:       clusterRes.Memory.Utilised,
			Recommended: recommendedMemGB,
		},
	})
}
