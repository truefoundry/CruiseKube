package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/types"
)

// HandleWorkloadDetail returns pod-level details for a single workload in one response,
// so the frontend Workload Detail page can use a single call instead of cluster-stats + recommendation-analysis.
// GET /api/v1/clusters/:clusterID/workloads/:namespace/:workloadName/detail
func HandleWorkloadDetail(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	workloadName := c.Param("workloadName")

	c.Header("Content-Type", "application/json")

	// 1. Get workload type (kind) from cluster stats
	var statsResponse types.StatsResponse
	since := time.Now().Add(-StatsAPIDataLookbackWindow)
	if err := storage.Stg.ReadClusterStatsUpdatedSince(clusterID, &statsResponse, since); err != nil {
		logging.Errorf(ctx, "Failed to read cluster stats for %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read cluster stats for %s: %v", clusterID, err),
		})
		return
	}

	workloadType := ""
	for i := range statsResponse.Stats {
		s := &statsResponse.Stats[i]
		if s.IsGPUWorkload() {
			continue
		}
		if s.Namespace == namespace && s.Name == workloadName {
			workloadType = s.Kind
			break
		}
	}

	// 2. Get recommendation analysis for the cluster
	mgr := c.MustGet("clusterManager").(cluster.Manager)
	analysisResponse, err := generateRecommendationAnalysisForCluster(ctx, clusterID, mgr)
	if err != nil {
		logging.Errorf(ctx, "Failed to generate recommendation analysis for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to generate recommendation analysis for %s: %v", clusterID, err),
		})
		return
	}

	// 3. Filter analysis items for this workload
	var items []types.RecommendationAnalysisItem
	for i := range analysisResponse.Analysis {
		item := &analysisResponse.Analysis[i]
		if item.WorkloadNamespace == namespace && item.WorkloadName == workloadName {
			items = append(items, *item)
		}
	}

	// 4. Build response: potentialCpu, potentialMem, pods
	resp := types.WorkloadDetailResponse{
		Type:         workloadType,
		PotentialCpu: 0,
		PotentialMem: 0,
		Pods:         nil,
	}

	if len(items) == 0 {
		resp.Pods = []types.PodDetail{}
		c.JSON(http.StatusOK, resp)
		return
	}

	var totalCpuDiff, totalMemDiff float64
	for i := range items {
		totalCpuDiff += items[i].CurrentRequestedCPU - items[i].RecommendedCPU
		totalMemDiff += items[i].CurrentRequestedMemory - items[i].RecommendedMemory
	}
	resp.PotentialCpu = -totalCpuDiff
	resp.PotentialMem = -totalMemDiff

	// 5. Build pods: unique pod names (sorted), with nodeName and containers
	podMap := make(map[string]*types.PodDetail)
	for i := range items {
		item := &items[i]
		pod, ok := podMap[item.PodName]
		if !ok {
			var nodeName *string
			if item.NodeName != "" {
				nodeName = &item.NodeName
			}
			pod = &types.PodDetail{
				PodName:    item.PodName,
				NodeName:   nodeName,
				Containers: nil,
			}
			podMap[item.PodName] = pod
		}
		if pod.NodeName == nil && item.NodeName != "" {
			n := item.NodeName
			pod.NodeName = &n
		}
		// One entry per container; deduplicate by container name
		hasContainer := false
		for _, co := range pod.Containers {
			if co.Container == item.ContainerName {
				hasContainer = true
				break
			}
		}
		if !hasContainer {
			pod.Containers = append(pod.Containers, types.ContainerDetail{
				Container:     item.ContainerName,
				CpuRequest:    item.CurrentRequestedCPU,
				CpuRecRequest: item.RecommendedCPU,
				MemRequest:    item.CurrentRequestedMemory,
				MemRecRequest: item.RecommendedMemory,
			})
		}
	}

	// Sort pod names and build slice
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
