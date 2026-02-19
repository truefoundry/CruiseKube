package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
)

type rowWithValues struct {
	row    types.PodResourceRecommendationRow
	rec    types.PodResourceRecommendation
	cpuReq float64
	memReq float64
	cpuRec float64
	memRec float64
}

// HandleWorkloadDetail returns pod-level details for a single workload in one response,
// using data from the database: workloads table (type, current requests) and
// pod_resource_recommendations table (recommended values, pod/container list).
// GET /api/v1/clusters/:clusterID/workloads/:namespace/:workloadName/detail
func HandleWorkloadDetail(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	workloadName := c.Param("workloadName")

	since := time.Now().Add(-StatsAPIDataLookbackWindow)

	// 1. Get workload (type + current container requests) from workloads table
	workloads, err := storage.Stg.GetWorkloadsInCluster(clusterID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get workloads for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get workloads for %s: %v", clusterID, err),
		})
		return
	}

	var workloadID string
	var stat *types.WorkloadStat
	for _, w := range workloads {
		s := w.GetStat()
		if s == nil || s.IsGPUWorkload() {
			continue
		}
		if s.Namespace == namespace && s.Name == workloadName {
			workloadID = w.WorkloadID
			stat = s
			break
		}
	}

	workloadType := ""
	if stat != nil {
		workloadType = stat.Kind
	}

	// 2. Get pod recommendations from pod_resource_recommendations table
	recRows, err := storage.Stg.GetPodRecommendationsForCluster(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get pod recommendations for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get pod recommendations for %s: %v", clusterID, err),
		})
		return
	}

	// Filter rows for this workload: by workloadID if we found it, else by parsing WorkloadID (namespace + name)
	var rows []types.PodResourceRecommendationRow
	for _, row := range recRows {
		if workloadID != "" {
			if row.WorkloadID != workloadID {
				continue
			}
		} else {
			_, rowNs, rowName, ok := utils.ParseWorkloadKey(row.WorkloadID)
			if !ok || rowNs != namespace || rowName != workloadName {
				continue
			}
		}
		rows = append(rows, row)
	}

	resp := types.WorkloadDetailResponse{
		Type:         workloadType,
		PotentialCpu: 0,
		PotentialMem: 0,
		Pods:         []types.PodDetail{},
	}

	if len(rows) == 0 {
		c.JSON(http.StatusOK, resp)
		return
	}

	// 3. Build per-row: current (from stat) and recommended (from row.Recommendation JSON)
	var parsed []rowWithValues
	for _, row := range rows {
		var rec types.PodResourceRecommendation
		if row.Recommendation != "" {
			if err := json.Unmarshal([]byte(row.Recommendation), &rec); err != nil {
				continue
			}
		}
		cpuRec := rec.CPURequest
		memRec := rec.MemoryRequest
		cpuReq := cpuRec
		memReq := memRec
		if stat != nil {
			if orig, err := stat.GetOriginalContainerResource(row.Container); err == nil {
				cpuReq = orig.CPURequest
				memReq = orig.MemoryRequest
				if row.Recommendation == "" {
					cpuRec = cpuReq
					memRec = memReq
				}
			}
		}
		parsed = append(parsed, rowWithValues{row: row, rec: rec, cpuReq: cpuReq, memReq: memReq, cpuRec: cpuRec, memRec: memRec})
	}

	// 4. potentialCpu / potentialMem
	var totalCpuDiff, totalMemDiff float64
	for _, p := range parsed {
		totalCpuDiff += p.cpuReq - p.cpuRec
		totalMemDiff += p.memReq - p.memRec
	}
	resp.PotentialCpu = math.Round(-totalCpuDiff*1000) / 1000
	resp.PotentialMem = math.Round(-totalMemDiff)

	// 5. Build pods: unique pod names (sorted), nodeName, containers
	podMap := make(map[string]*types.PodDetail)
	for _, p := range parsed {
		row := p.row
		pod, ok := podMap[row.Pod]
		if !ok {
			var nodeName *string
			if row.NodeName != "" {
				nodeName = &row.NodeName
			}
			pod = &types.PodDetail{
				PodName:    row.Pod,
				NodeName:   nodeName,
				Containers: nil,
			}
			podMap[row.Pod] = pod
		}
		if pod.NodeName == nil && row.NodeName != "" {
			n := row.NodeName
			pod.NodeName = &n
		}
		hasContainer := false
		for _, co := range pod.Containers {
			if co.Container == row.Container {
				hasContainer = true
				break
			}
		}
		if !hasContainer {
			// Apply same rounding as recommendation-analysis API (analyzeWorkloadStats) so values match previous response
			cpuReqRounded := math.Round(p.cpuReq*1000) / 1000
			cpuRecRounded := math.Round(p.cpuRec*1000) / 1000
			memReqRounded := math.Round(p.memReq)
			memRecRounded := math.Round(p.memRec)
			pod.Containers = append(pod.Containers, types.ContainerDetail{
				Container:     row.Container,
				CpuRequest:    cpuReqRounded,
				CpuRecRequest: cpuRecRounded,
				MemRequest:    memReqRounded,
				MemRecRequest: memRecRounded,
			})
		}
	}

	podNames := make([]string, 0, len(podMap))
	for name := range podMap {
		podNames = append(podNames, name)
	}
	sort.Strings(podNames)
	resp.Pods = make([]types.PodDetail, 0, len(podNames))
	for _, name := range podNames {
		resp.Pods = append(resp.Pods, *podMap[name])
	}

	c.JSON(http.StatusOK, resp)
}
