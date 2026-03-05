package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
)

const (
	defaultCPUPricePerCorePerHour  = 0.0145
	defaultMemoryPricePerGBPerHour = 0.00725
	defaultHoursPerMonth           = 720
)

type workloadPricing struct {
	CPUPerCorePerHour float64
	MemPerGBPerHour   float64
}

func (deps HandlerDependencies) getEffectivePricing(ctx context.Context, clusterID string) workloadPricing {
	p := workloadPricing{
		CPUPerCorePerHour: defaultCPUPricePerCorePerHour,
		MemPerGBPerHour:   defaultMemoryPricePerGBPerHour,
	}
	if deps.Storage == nil {
		return p
	}
	settings, err := deps.Storage.GetSettings(clusterID)
	if err != nil {
		logging.Warnf(ctx, "Failed to get settings for cluster %s, using defaults: %v", clusterID, err)
		return p
	}
	if settings == nil {
		return p
	}
	if settings.CPUPricePerCorePerHour > 0 {
		p.CPUPerCorePerHour = settings.CPUPricePerCorePerHour
	}
	if settings.MemoryPricePerGBPerHour > 0 {
		p.MemPerGBPerHour = settings.MemoryPricePerGBPerHour
	}
	return p
}

func (deps HandlerDependencies) getClusterResourcesFromPrometheus(ctx context.Context, clusterID string) types.ClusterResourcesDTO {
	out := types.ClusterResourcesDTO{
		CPU:    types.ClusterResourceDTO{Utilised: 0, Requested: 0, Allocatable: 0},
		Memory: types.ClusterResourceDTO{Utilised: 0, Requested: 0, Allocatable: 0},
	}
	if deps.ClusterManager == nil {
		return out
	}
	clients, err := deps.ClusterManager.GetClusterClients(clusterID)
	if err != nil || clients == nil || clients.PrometheusClient == nil {
		return out
	}
	pc := clients.PrometheusClient
	out.CPU.Utilised = utils.QueryAndParsePrometheusScalar(ctx, pc, utils.BuildClusterCPUUtilizationExpression())
	out.CPU.Requested = utils.QueryAndParsePrometheusScalar(ctx, pc, utils.BuildClusterCPURequestExpression())
	out.CPU.Allocatable = utils.QueryAndParsePrometheusScalar(ctx, pc, utils.BuildClusterCPUAllocatableExpression())
	out.Memory.Utilised = utils.QueryAndParsePrometheusScalar(ctx, pc, utils.BuildClusterMemoryUtilizationExpression())
	out.Memory.Requested = utils.QueryAndParsePrometheusScalar(ctx, pc, utils.BuildClusterMemoryRequestExpression())
	out.Memory.Allocatable = utils.QueryAndParsePrometheusScalar(ctx, pc, utils.BuildClusterMemoryAllocatableExpression())
	return out
}

// getNonGPUClusterWorkloads returns workloads for the cluster (single DB call), filtered to non-GPU.
func (deps HandlerDependencies) getNonGPUClusterWorkloads(ctx context.Context, clusterID string) ([]*types.WorkloadInCluster, error) {
	workloads, err := deps.Storage.GetWorkloadsInCluster(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get workloads for cluster %s: %v", clusterID, err)
		return nil, fmt.Errorf("get workloads for cluster %s: %w", clusterID, err)
	}
	// filter out GPU workloads
	out := make([]*types.WorkloadInCluster, 0, len(workloads))
	for _, w := range workloads {
		if w.GetStat() != nil && !w.GetStat().IsGPUWorkload() {
			out = append(out, w)
		}
	}
	return out, nil
}

type parsedPodRecommendation struct {
	WorkloadID string
	Namespace  string
	Pod        string
	Rec        types.PodResourceRecommendation
}

type workloadRecAgg struct {
	CPUMin   float64
	CPUMax   float64
	MemMin   float64
	MemMax   float64
	TotalCPU float64
	TotalMem float64
}

func (deps HandlerDependencies) getPodRecommendationsForCluster(ctx context.Context, clusterID string) ([]parsedPodRecommendation, error) {
	recRows, err := deps.Storage.GetPodRecommendationsForCluster(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get pod recommendations for cluster %s: %v", clusterID, err)
		return nil, fmt.Errorf("get pod recommendations for cluster %s: %w", clusterID, err)
	}
	parsedRecs := make([]parsedPodRecommendation, 0, len(recRows))
	for _, row := range recRows {
		var rec types.PodResourceRecommendation
		if err := json.Unmarshal([]byte(row.Recommendation), &rec); err != nil {
			continue
		}
		parsedRecs = append(parsedRecs, parsedPodRecommendation{WorkloadID: row.WorkloadID, Namespace: row.Namespace, Pod: row.Pod, Rec: rec})
	}
	return parsedRecs, nil
}

// aggregateRecsForWorkload aggregates pod recommendations for a single workload into min/max/total CPU and memory.
// Assumes at most one row per pod per workload (no duplicate pod names in recs).
func aggregateRecsForWorkload(recs []parsedPodRecommendation) workloadRecAgg {
	var cpuMin, cpuMax, memMin, memMax, totalCPU, totalMem float64
	first := true
	for _, p := range recs {
		cpu, mem := p.Rec.CPURequest, p.Rec.MemoryRequest
		totalCPU += cpu
		totalMem += mem
		if first {
			cpuMin, cpuMax = cpu, cpu
			memMin, memMax = mem, mem
			first = false
		} else {
			if cpu < cpuMin {
				cpuMin = cpu
			}
			if cpu > cpuMax {
				cpuMax = cpu
			}
			if mem < memMin {
				memMin = mem
			}
			if mem > memMax {
				memMax = mem
			}
		}
	}
	return workloadRecAgg{CPUMin: cpuMin, CPUMax: cpuMax, MemMin: memMin, MemMax: memMax, TotalCPU: totalCPU, TotalMem: totalMem}
}

// buildWorkloadDetail builds a single WorkloadDetail from a workload and its stat.
func buildWorkloadDetail(w *types.WorkloadInCluster, stat *types.WorkloadStat) types.WorkloadDetail {
	effective := w.OverridesWithDefaults()
	priority := "medium"
	switch effective.EvictionRanking {
	case types.EvictionRankingDisabled:
		priority = "non-evictable"
	case types.EvictionRankingLow:
		priority = "low"
	case types.EvictionRankingMedium:
		priority = "medium"
	case types.EvictionRankingHigh:
		priority = "high"
	}
	var constraints types.WorkloadSummaryConstraints
	if stat.Constraints != nil {
		constraints = types.WorkloadSummaryConstraints{
			BlockingConsolidation:    stat.Constraints.BlockingConsolidation,
			PDB:                      stat.Constraints.PDB,
			DoNotDisruptAnnotation:   stat.Constraints.DoNotDisruptAnnotation,
			Volume:                   stat.Constraints.Volume,
			Affinity:                 stat.Constraints.Affinity,
			TopologySpreadConstraint: stat.Constraints.TopologySpreadConstraint,
			PodAntiAffinity:          stat.Constraints.PodAntiAffinity,
			ExcludedAnnotation:       stat.Constraints.ExcludedAnnotation,
		}
	}
	disruptionSchedule := make([]types.DisruptionScheduleWindow, 0, len(effective.DisruptionWindows))
	if len(effective.DisruptionWindows) > 0 {
		for _, wdw := range effective.DisruptionWindows {
			disruptionSchedule = append(disruptionSchedule, types.DisruptionScheduleWindow{
				WindowStartCron: wdw.StartCron,
				WindowEndCron:   wdw.EndCron,
			})
		}
	}
	inDisruptionWindow := stat.Metadata != nil && stat.Metadata.InDisruptionWindow
	return types.WorkloadDetail{
		WorkloadID:  w.WorkloadID,
		Kind:        stat.Kind,
		Namespace:   stat.Namespace,
		Name:        stat.Name,
		UpdatedAt:   stat.UpdatedAt.Unix(),
		PodsCount:   int(stat.Replicas),
		Constraints: constraints,
		Config: types.WorkloadConfig{
			Priority:           priority,
			CruiseEnabled:      effective.Enabled,
			DisruptionSchedule: disruptionSchedule,
			InDisruptionWindow: inDisruptionWindow,
		},
	}
}

// fillWorkloadDetailDollars sets dollar savings and expenditure on a single WorkloadDetail from aggregated recommendations.
// d.CPU.Current and d.Memory.Current are expected to be totals (per-pod request * number of pods).
func fillWorkloadDetailDollars(d *types.WorkloadDetail, agg workloadRecAgg, p workloadPricing) {
	totalCurrentCPU := d.CPU.CurrentPerPod * float64(d.PodsCount)
	totalCurrentMem := d.Memory.CurrentPerPod * float64(d.PodsCount)
	totalRecCPU := agg.TotalCPU
	totalRecMem := agg.TotalMem
	cpuSavings := 0.0
	if totalCurrentCPU > totalRecCPU {
		cpuSavings = (totalCurrentCPU - totalRecCPU) * p.CPUPerCorePerHour * defaultHoursPerMonth
	}
	memSavings := 0.0
	if totalCurrentMem > totalRecMem {
		memSavings = (totalCurrentMem - totalRecMem) / 1024 * p.MemPerGBPerHour * defaultHoursPerMonth
	}
	d.DollarSavingsPerMonth = int(cpuSavings + memSavings)
	cpuExpenditure := 0.0
	if totalRecCPU > totalCurrentCPU {
		cpuExpenditure = (totalRecCPU - totalCurrentCPU) * p.CPUPerCorePerHour * defaultHoursPerMonth
	}
	memExpenditure := 0.0
	if totalRecMem > totalCurrentMem {
		memExpenditure = (totalRecMem - totalCurrentMem) / 1024 * p.MemPerGBPerHour * defaultHoursPerMonth
	}
	d.DollarExpenditurePerMonth = int(cpuExpenditure + memExpenditure)
}

// getWorkloadsData fetches non-GPU workloads and pod recommendations for a cluster, then for each workload
// filters recommendations by workload ID, computes total CPU, memory, cost, and attaches everything to
// WorkloadDetail. Returns details and cluster-level requested/recommended CPU and memory.
func (deps HandlerDependencies) getWorkloadsData(ctx context.Context, clusterID string) ([]types.WorkloadDetail, map[string]workloadRecAgg, float64, float64, float64, float64, error) {
	workloads, err := deps.getNonGPUClusterWorkloads(ctx, clusterID)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	parsedRecs, err := deps.getPodRecommendationsForCluster(ctx, clusterID)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	// Index recommendations by workload ID for fast lookup
	recsByWorkload := make(map[string][]parsedPodRecommendation)
	for _, p := range parsedRecs {
		recsByWorkload[p.WorkloadID] = append(recsByWorkload[p.WorkloadID], p)
	}

	var clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem float64
	details := make([]types.WorkloadDetail, 0, len(workloads))
	recAgg := make(map[string]workloadRecAgg, len(workloads))
	for _, w := range workloads {
		stat := w.GetStat()
		if stat == nil {
			continue
		}
		detail := buildWorkloadDetail(w, stat)
		agg := aggregateRecsForWorkload(recsByWorkload[w.WorkloadID])
		recAgg[w.WorkloadID] = agg

		workloadPodCPURequest := stat.CalculateTotalCPURequest()
		workloadTotalMemoryRequest := stat.CalculateTotalMemoryRequest()
		replicas := stat.Replicas
		if replicas <= 0 {
			replicas = 1
		}
		// Per Pod Current Request
		currentCPUPerPod := workloadPodCPURequest
		currentMemPerPod := workloadTotalMemoryRequest

		////////////////////////////////////////////////////////////
		// Added to global cluster request and recommendation
		////////////////////////////////////////////////////////////
		clusterReqCPU += currentCPUPerPod * float64(replicas)
		clusterReqMem += currentMemPerPod * float64(replicas)
		clusterRecCPU += agg.TotalCPU
		clusterRecMem += agg.TotalMem

		// The difference needs to be per pod
		cpuChange := (agg.TotalCPU / float64(replicas)) - currentCPUPerPod
		memChange := (agg.TotalMem / float64(replicas)) - currentMemPerPod
		if stat.Replicas <= 0 {
			cpuChange, memChange = 0, 0
		}

		detail.CPU = types.WorkloadCPU{
			CurrentPerPod: currentCPUPerPod,
			Recommended: types.CPURecommended{
				Min: agg.CPUMin, Max: agg.CPUMax, Change: cpuChange,
			},
		}
		detail.Memory = types.WorkloadMemory{
			CurrentPerPod: currentMemPerPod,
			Recommended: types.MemoryRecommended{
				Min: agg.MemMin, Max: agg.MemMax, Change: memChange,
			},
		}
		details = append(details, detail)
	}

	return details, recAgg, clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem, nil
}

func (deps HandlerDependencies) WorkloadSummaryHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	details, recAgg, clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem, err := deps.getWorkloadsData(ctx, clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	p := deps.getEffectivePricing(ctx, clusterID)
	for i := range details {
		fillWorkloadDetailDollars(&details[i], recAgg[details[i].WorkloadID], p)
	}

	clusterRes := deps.getClusterResourcesFromPrometheus(ctx, clusterID)
	reqAllocRatioCpu := 1.0
	if clusterRes.CPU.Allocatable > 0 {
		reqAllocRatioCpu = clusterRes.CPU.Requested / clusterRes.CPU.Allocatable
	}
	reqAllocRatioMem := 1.0
	if clusterRes.Memory.Allocatable > 0 {
		reqAllocRatioMem = clusterRes.Memory.Requested / clusterRes.Memory.Allocatable
	}
	requestedMemGB := clusterReqMem / 1024
	recommendedMemGB := clusterRecMem / 1024
	currentCostDollars := (clusterRes.CPU.Allocatable*p.CPUPerCorePerHour + clusterRes.Memory.Allocatable*p.MemPerGBPerHour) * defaultHoursPerMonth
	workloadCostDollars := (clusterReqCPU/reqAllocRatioCpu)*p.CPUPerCorePerHour*defaultHoursPerMonth + (requestedMemGB/reqAllocRatioMem)*p.MemPerGBPerHour*defaultHoursPerMonth
	optimizedCostDollars := (clusterRecCPU/reqAllocRatioCpu)*p.CPUPerCorePerHour*defaultHoursPerMonth + (recommendedMemGB/reqAllocRatioMem)*p.MemPerGBPerHour*defaultHoursPerMonth
	c.JSON(http.StatusOK, types.WorkloadSummaryResponse{
		ImpactSummary: types.ImpactSummary{
			DollarCurrentCost:     int(currentCostDollars),
			DollarCurrentSavings:  int(workloadCostDollars - currentCostDollars),
			DollarPossibleSavings: int(workloadCostDollars - optimizedCostDollars),
			ClusterResources:      clusterRes,
		},
		WorkloadDetails: details,
	})
}
