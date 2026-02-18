package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/types"
)

const (
	defaultCPUPricePerCorePerHour  = 0.0145
	defaultMemoryPricePerGbPerHour = 0.00725
	defaultHoursPerMonth           = 720
)

var prometheusClusterQueries = struct {
	CPUUtilised, CPURequested, CPUAllocatable          string
	MemoryUtilised, MemoryRequested, MemoryAllocatable string
}{
	CPUUtilised: `round(
      sum(
        sum by (node) (
          rate(node_cpu_seconds_total{job="node-exporter", mode=~"user|system"}[1m])
        )
        unless max by (node) (
          max_over_time(kube_node_status_allocatable{
            job="kube-state-metrics",
            resource=~"nvidia_com_gpu|amd_com_gpu"
          }[7d:]) > 0
        )
      ),
      0.001
    )`,
	CPURequested: `round(
      sum(
        sum by (node) (
          (
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource="cpu"})
            )
            unless on (namespace, pod)
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource=~"nvidia_com_gpu|amd_com_gpu"})
            )
          )
          * on (namespace, pod) group_left
            sum by (namespace, pod) (kube_pod_status_phase{job="kube-state-metrics", phase!~"Failed|Succeeded|Unknown|Pending"})
        )
        unless on (node)
        (
          max by (node) (
            max_over_time(
              kube_node_status_allocatable{job="kube-state-metrics", resource=~"nvidia_com_gpu|amd_com_gpu"}[7d:]
            )
          )
          >
          0
        )
      ),
      0.001
    )`,
	CPUAllocatable: `round(
      sum(
        sum by (node) (kube_node_status_allocatable{job="kube-state-metrics", resource="cpu"})
        unless (
          sum by (node) (
            kube_node_spec_taint{job="kube-state-metrics", key="nvidia.com/gpu"}
          )
        )
        unless on (node) (
          kube_node_labels{job="kube-state-metrics", accelerator="nvidia"}
        )
      ),
      0.001
    )`,
	MemoryUtilised: `round(
      sum(
        sum by (node) (
          node_memory_MemTotal_bytes{job="node-exporter"} - (node_memory_MemFree_bytes{job="node-exporter"} + node_memory_Buffers_bytes{job="node-exporter"} + node_memory_Cached_bytes{job="node-exporter"})
        )
        unless
        max by (node) (
          max_over_time(kube_node_status_allocatable{job="kube-state-metrics", resource=~"nvidia_com_gpu|amd_com_gpu"}[7d:])
        ) > 0
      )
      / 1000000000,
      0.001
    )`,
	MemoryRequested: `round(
      sum(
        sum by (node) (
          (
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource="memory"})
            )
            unless on (namespace, pod)
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource=~"nvidia_com_gpu|amd_com_gpu"})
            )
          )
          * on (namespace, pod) group_left
            sum by (namespace, pod) (kube_pod_status_phase{job="kube-state-metrics", phase!~"Failed|Succeeded|Unknown|Pending"})
        )
        unless on (node)
        (
          max by (node) (
            max_over_time(
              kube_node_status_allocatable{job="kube-state-metrics", resource=~"nvidia_com_gpu|amd_com_gpu"}[7d:]
            )
          )
          >
          0
        )
      ) / 1000000000,
      0.001
    )`,
	MemoryAllocatable: `round(
      sum(
        sum by (node) (kube_node_status_allocatable{job="kube-state-metrics", resource="memory"})
        unless (
          sum by (node) (kube_node_spec_taint{job="kube-state-metrics", key="nvidia.com/gpu"})
        )
        unless on (node) (
          kube_node_labels{job="kube-state-metrics", accelerator="nvidia"}
        )
      ) / 1000000000,
      0.001
    )`,
}

func queryPrometheusScalar(ctx context.Context, client v1.API, q string) float64 {
	if client == nil {
		return 0
	}
	result, _, err := client.Query(ctx, q, time.Now())
	if err != nil || result == nil {
		return 0
	}
	if v, ok := result.(model.Vector); ok && len(v) > 0 {
		return float64(v[0].Value)
	}
	if s, ok := result.(*model.Scalar); ok {
		return float64(s.Value)
	}
	return 0
}

func getClusterResourcesFromPrometheus(ctx context.Context, c *gin.Context, clusterID string) types.ClusterResourcesDTO {
	out := types.ClusterResourcesDTO{
		CPU:    types.ClusterResourceDTO{Utilised: 0, Requested: 0, Allocatable: 0},
		Memory: types.ClusterResourceDTO{Utilised: 0, Requested: 0, Allocatable: 0},
	}
	mgr, _ := c.Get("clusterManager")
	if mgr == nil {
		return out
	}
	clients, err := mgr.(cluster.Manager).GetClusterClients(clusterID)
	if err != nil || clients == nil || clients.PrometheusClient == nil {
		return out
	}
	pc := clients.PrometheusClient
	q := prometheusClusterQueries
	out.CPU.Utilised = queryPrometheusScalar(ctx, pc, q.CPUUtilised)
	out.CPU.Requested = queryPrometheusScalar(ctx, pc, q.CPURequested)
	out.CPU.Allocatable = queryPrometheusScalar(ctx, pc, q.CPUAllocatable)
	out.Memory.Utilised = queryPrometheusScalar(ctx, pc, q.MemoryUtilised)
	out.Memory.Requested = queryPrometheusScalar(ctx, pc, q.MemoryRequested)
	out.Memory.Allocatable = queryPrometheusScalar(ctx, pc, q.MemoryAllocatable)
	return out
}

func filterNonGPUStats(stats []types.WorkloadStat) []types.WorkloadStat {
	out := make([]types.WorkloadStat, 0, len(stats))
	for i := range stats {
		if !stats[i].IsGPUWorkload() {
			out = append(out, stats[i])
		}
	}
	return out
}

func getFilteredClusterStats(ctx context.Context, clusterID string) ([]types.WorkloadStat, error) {
	var statsResponse types.StatsResponse
	since := time.Now().Add(-StatsAPIDataLookbackWindow)
	if err := storage.Stg.ReadClusterStatsUpdatedSince(clusterID, &statsResponse, since); err != nil {
		logging.Errorf(ctx, "Failed to read cluster stats for %s: %v", clusterID, err)
		return nil, fmt.Errorf("read cluster stats for %s: %w", clusterID, err)
	}
	return filterNonGPUStats(statsResponse.Stats), nil
}

func getWorkloadDetails(ctx context.Context, clusterID string, stats []types.WorkloadStat) ([]types.WorkloadDetail, error) {
	details := make([]types.WorkloadDetail, 0, len(stats))
	for _, stat := range stats {
		workloadColumnId := strings.ReplaceAll(stat.WorkloadIdentifier, "/", ":")
		overrides, err := storage.Stg.GetWorkloadOverrides(clusterID, workloadColumnId)
		if err != nil {
			logging.Errorf(ctx, "Failed to get overrides for workload %s: %v", workloadColumnId, err)
			return nil, fmt.Errorf("get overrides for workload %s: %w", workloadColumnId, err)
		}
		evictionRanking := stat.EvictionRanking
		enabled := true
		if overrides != nil {
			if overrides.EvictionRanking != nil {
				evictionRanking = *overrides.EvictionRanking
			}
			if overrides.Enabled != nil {
				enabled = *overrides.Enabled
			}
		}
		priority := "medium"
		switch evictionRanking {
		case types.EvictionRankingDisabled:
			priority = "non-evictable"
		case types.EvictionRankingLow:
			priority = "low"
		case types.EvictionRankingMedium:
			priority = "medium"
		case types.EvictionRankingHigh:
			priority = "high"
		}
		mode := "recommend-only"
		if enabled {
			mode = "enabled"
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
		details = append(details, types.WorkloadDetail{
			WorkloadID:  workloadColumnId,
			Kind:        stat.Kind,
			Namespace:   stat.Namespace,
			Name:        stat.Name,
			UpdatedAt:   stat.UpdatedAt.Unix(),
			PodsCount:   int(stat.Replicas),
			Constraints: constraints,
			Config: types.WorkloadConfig{
				Priority:           priority,
				Mode:               mode,
				DisruptionSchedule: []types.DisruptionScheduleWindow{},
			},
		})
	}
	return details, nil
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

func getPodRecommendationsForCluster(ctx context.Context, clusterID string) ([]parsedPodRecommendation, error) {
	recRows, err := storage.Stg.GetPodRecommendationsForCluster(clusterID)
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

func aggregateRecommendationsByWorkload(parsedRecs []parsedPodRecommendation) map[string]workloadRecAgg {
	byWorkloadPod := make(map[string]map[string]struct{ CPU, Mem float64 })
	for _, p := range parsedRecs {
		wid := p.WorkloadID
		if byWorkloadPod[wid] == nil {
			byWorkloadPod[wid] = make(map[string]struct{ CPU, Mem float64 })
		}
		podTotals := byWorkloadPod[wid][p.Pod]
		podTotals.CPU += p.Rec.CPURequest
		podTotals.Mem += p.Rec.MemoryRequest
		byWorkloadPod[wid][p.Pod] = podTotals
	}
	out := make(map[string]workloadRecAgg)
	for wid, podMap := range byWorkloadPod {
		var cpuMin, cpuMax, memMin, memMax, totalCPU, totalMem float64
		first := true
		for _, tot := range podMap {
			totalCPU += tot.CPU
			totalMem += tot.Mem
			if first {
				cpuMin, cpuMax = tot.CPU, tot.CPU
				memMin, memMax = tot.Mem, tot.Mem
				first = false
			} else {
				if tot.CPU < cpuMin {
					cpuMin = tot.CPU
				}
				if tot.CPU > cpuMax {
					cpuMax = tot.CPU
				}
				if tot.Mem < memMin {
					memMin = tot.Mem
				}
				if tot.Mem > memMax {
					memMax = tot.Mem
				}
			}
		}
		out[wid] = workloadRecAgg{CPUMin: cpuMin, CPUMax: cpuMax, MemMin: memMin, MemMax: memMax, TotalCPU: totalCPU, TotalMem: totalMem}
	}
	return out
}

func clusterRequestedFromStats(stats []types.WorkloadStat) (float64, float64) {
	var cpu, mem float64
	for i := range stats {
		replicas := float64(stats[i].Replicas)
		if replicas <= 0 {
			replicas = 1
		}
		cpu += stats[i].CalculateTotalCPURequest() * replicas
		mem += stats[i].CalculateTotalMemoryRequest() * replicas
	}
	return cpu, mem
}

func clusterRecommendedFromRecs(parsedRecs []parsedPodRecommendation) (float64, float64) {
	var cpu, mem float64
	for _, p := range parsedRecs {
		cpu += p.Rec.CPURequest
		mem += p.Rec.MemoryRequest
	}
	return cpu, mem
}

func fillWorkloadDetailsWithResources(stats []types.WorkloadStat, details []types.WorkloadDetail, recAgg map[string]workloadRecAgg, parsedRecs []parsedPodRecommendation) (float64, float64, float64, float64) {
	var clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem float64
	clusterReqCPU, clusterReqMem = clusterRequestedFromStats(stats)
	clusterRecCPU, clusterRecMem = clusterRecommendedFromRecs(parsedRecs)
	for i := range details {
		stat := stats[i]
		currentCPU := stat.CalculateTotalCPURequest()
		currentMem := stat.CalculateTotalMemoryRequest()
		agg := recAgg[details[i].WorkloadID]
		details[i].CPU = types.WorkloadCPU{
			Current: currentCPU,
			Recommended: types.CPURecommended{
				Min: agg.CPUMin, Max: agg.CPUMax, Change: agg.CPUMax - currentCPU,
			},
		}
		details[i].Memory = types.WorkloadMemory{
			Current: currentMem,
			Recommended: types.MemoryRecommended{
				Min: agg.MemMin, Max: agg.MemMax, Change: agg.MemMax - currentMem,
			},
		}
	}
	return clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem
}

func fillWorkloadDetailsDollars(details []types.WorkloadDetail, recAgg map[string]workloadRecAgg) {
	for i := range details {
		d := &details[i]
		agg := recAgg[d.WorkloadID]
		podsCount := float64(d.PodsCount)
		if podsCount <= 0 {
			podsCount = 1
		}
		totalCurrentCPU := d.CPU.Current * podsCount
		totalCurrentMem := d.Memory.Current * podsCount
		totalRecCPU := agg.TotalCPU
		totalRecMem := agg.TotalMem
		cpuSavings := 0.0
		if totalCurrentCPU > totalRecCPU {
			cpuSavings = (totalCurrentCPU - totalRecCPU) * defaultCPUPricePerCorePerHour * defaultHoursPerMonth
		}
		memSavings := 0.0
		if totalCurrentMem > totalRecMem {
			memSavings = (totalCurrentMem - totalRecMem) / 1024 * defaultMemoryPricePerGbPerHour * defaultHoursPerMonth
		}
		d.DollarSavingsPerMonth = int(cpuSavings + memSavings)
		cpuExpenditure := 0.0
		if totalRecCPU > totalCurrentCPU {
			cpuExpenditure = (totalRecCPU - totalCurrentCPU) * defaultCPUPricePerCorePerHour * defaultHoursPerMonth
		}
		memExpenditure := 0.0
		if totalRecMem > totalCurrentMem {
			memExpenditure = (totalRecMem - totalCurrentMem) / 1024 * defaultMemoryPricePerGbPerHour * defaultHoursPerMonth
		}
		d.DollarExpenditurePerMonth = int(cpuExpenditure + memExpenditure)
	}
}

func WorkloadSummaryHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	stats, err := getFilteredClusterStats(ctx, clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	details, err := getWorkloadDetails(ctx, clusterID, stats)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	parsedRecs, err := getPodRecommendationsForCluster(ctx, clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recAgg := aggregateRecommendationsByWorkload(parsedRecs)
	clusterReqCPU, clusterReqMem, clusterRecCPU, clusterRecMem := fillWorkloadDetailsWithResources(stats, details, recAgg, parsedRecs)
	fillWorkloadDetailsDollars(details, recAgg)
	clusterRes := getClusterResourcesFromPrometheus(ctx, c, clusterID)
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
	currentCostDollars := (clusterRes.CPU.Allocatable*defaultCPUPricePerCorePerHour + clusterRes.Memory.Allocatable*defaultMemoryPricePerGbPerHour) * defaultHoursPerMonth
	workloadCostDollars := (clusterReqCPU/reqAllocRatioCpu)*defaultCPUPricePerCorePerHour*defaultHoursPerMonth + (requestedMemGB/reqAllocRatioMem)*defaultMemoryPricePerGbPerHour*defaultHoursPerMonth
	optimizedCostDollars := (clusterRecCPU/reqAllocRatioCpu)*defaultCPUPricePerCorePerHour*defaultHoursPerMonth + (recommendedMemGB/reqAllocRatioMem)*defaultMemoryPricePerGbPerHour*defaultHoursPerMonth
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
