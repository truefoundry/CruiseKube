package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/task"
	"github.com/truefoundry/cruisekube/pkg/task/applystrategies"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
)

const (
	YesValue = "Yes"
	NoValue  = "No"
)

func (deps HandlerDependencies) RecommendationAnalysisHandlerForCluster(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	clusterID := c.Param("clusterID")
	response, err := generateRecommendationAnalysisForCluster(c.Request.Context(), clusterID, deps.ClusterManager)
	if err != nil {
		logging.Errorf(c.Request.Context(), "Failed to generate recommendation analysis for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to generate recommendation analysis for cluster %s: %v", clusterID, err),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func generateRecommendationAnalysisForCluster(ctx context.Context, clusterID string, clusterMgr cluster.Manager) (*types.RecommendationAnalysisResponse, error) {
	clusterTask, err := clusterMgr.GetTask(clusterID + "_" + config.ApplyRecommendationKey)
	if err != nil {
		return nil, fmt.Errorf("error getting recommendation task: %w", err)
	}

	recomTask := clusterTask.GetCoreTask().(*task.ApplyRecommendationTask)
	nodeRecommendationMap, err := recomTask.GenerateNodeStatsForCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("error generating node recommendations: %w", err)
	}

	recommendationResults, err := recomTask.ApplyRecommendationsWithStrategy(ctx, nodeRecommendationMap, nil, applystrategies.NewAdjustAmongstPodsDistributedStrategy(ctx), false, true, false)
	if err != nil {
		return nil, fmt.Errorf("error applying recommendations: %w", err)
	}

	var analysis []types.RecommendationAnalysisItem
	var totalCurrentRequests, totalDifferences, totalCurrentMemoryRequests, totalMemoryDifferences float64

	for _, result := range recommendationResults {
		for _, rec := range result.PodContainerRecommendations {
			if rec.PodInfo.Stats != nil && !rec.PodInfo.Stats.IsGPUWorkload() {
				containerResource, err := rec.PodInfo.GetOriginalContainerResource(rec.ContainerName)
				if err != nil {
					logging.Errorf(ctx, "error getting container resource for pod %s/%s: %v", rec.PodInfo.Namespace, rec.PodInfo.Name, err)
					continue
				}
				currentRequestedCPU := containerResource.CPURequest
				currentRequestedMemory := containerResource.MemoryRequest
				analysisItem := analyzeWorkloadStats(ctx, rec.PodInfo.Stats, rec.PodInfo.Name, rec.ContainerName, result.NodeName, currentRequestedCPU, rec.CPU, currentRequestedMemory, rec.Memory)
				analysis = append(analysis, analysisItem)
				totalCurrentRequests += currentRequestedCPU
				totalDifferences += analysisItem.CPUDifference
				totalCurrentMemoryRequests += currentRequestedMemory
				totalMemoryDifferences += analysisItem.MemoryDifference
			}
		}
	}

	summary := types.RecommendationSummary{
		TotalCurrentCPURequests:    totalCurrentRequests,
		TotalCPUDifferences:        totalDifferences,
		TotalCurrentMemoryRequests: totalCurrentMemoryRequests,
		TotalMemoryDifferences:     totalMemoryDifferences,
	}

	return &types.RecommendationAnalysisResponse{
		Analysis: analysis,
		Summary:  summary,
	}, nil
}

func analyzeWorkloadStats(ctx context.Context, stat *utils.WorkloadStat, podName, containerName, nodeName string, currentRequestedCPU, recommendedCPU, currentRequestedMemory, recommendedMemory float64) types.RecommendationAnalysisItem {
	blockingKarpenter := NoValue
	if stat.Constraints != nil && stat.Constraints.BlockingConsolidation {
		blockingKarpenter = YesValue
	}

	containerStat, err := stat.GetContainerStats(containerName)
	if err != nil {
		logging.Warnf(ctx, "Failed to get container stats for %s/%s container %s: %v", stat.Namespace, stat.Name, containerName, err)
	}

	spikeRange := 0.0
	if containerStat != nil && containerStat.SimplePredictionsCPU != nil && containerStat.CPUStats != nil {
		spikeRange = containerStat.SimplePredictionsCPU.MaxValue - containerStat.CPUStats.P50
	}

	requestGap := 0.0
	if containerStat != nil && containerStat.CPUStats != nil {
		requestGap = currentRequestedCPU - containerStat.CPUStats.P50
	}

	cpuDifference := math.Round((currentRequestedCPU-recommendedCPU)*1000) / 1000
	memoryDifference := math.Round(currentRequestedMemory - recommendedMemory)

	return types.RecommendationAnalysisItem{
		WorkloadType:           stat.Kind,
		WorkloadNamespace:      stat.Namespace,
		WorkloadName:           stat.Name,
		ContainerName:          containerName,
		PodName:                podName,
		SpikeRange:             spikeRange,
		RequestGap:             requestGap,
		BlockingKarpenter:      blockingKarpenter,
		NodeName:               nodeName,
		CurrentRequestedCPU:    math.Round(currentRequestedCPU*1000) / 1000,
		RecommendedCPU:         math.Round(recommendedCPU*1000) / 1000,
		CPUDifference:          cpuDifference,
		CurrentRequestedMemory: math.Round(currentRequestedMemory),
		RecommendedMemory:      math.Round(recommendedMemory),
		MemoryDifference:       math.Round(memoryDifference),
	}
}
