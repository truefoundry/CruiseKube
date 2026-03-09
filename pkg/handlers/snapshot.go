package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func parseUTCTimestamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		// Support both seconds and milliseconds UNIX timestamps.
		if unix > 1_000_000_000_000 {
			return time.UnixMilli(unix).UTC(), nil
		}
		return time.Unix(unix, 0).UTC(), nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid UTC timestamp: %s", raw)
}

func buildCostFromSnapshot(p workloadPricing, snapshot types.SnapshotRecord) (float64, float64, float64) {
	reqAllocRatioCPU := 1.0
	if snapshot.Data.CPU.CurrentAllocatable > 0 && snapshot.Data.CPU.CurrentRequested > 0 {
		reqAllocRatioCPU = snapshot.Data.CPU.CurrentRequested / snapshot.Data.CPU.CurrentAllocatable
	}
	reqAllocRatioMem := 1.0
	if snapshot.Data.Memory.CurrentAllocatable > 0 && snapshot.Data.Memory.CurrentRequested > 0 {
		reqAllocRatioMem = snapshot.Data.Memory.CurrentRequested / snapshot.Data.Memory.CurrentAllocatable
	}
	requestedMemGB := snapshot.Data.Memory.WorkloadRequested
	recommendedMemGB := snapshot.Data.Memory.RecommendedRequested

	// Cost series values are per hour.
	currentCost := snapshot.Data.CPU.CurrentAllocatable*p.CPUPerCorePerHour + snapshot.Data.Memory.CurrentAllocatable*p.MemPerGBPerHour
	// Cost if infra were sized to workload-requested resources (Without CruiseKube), per hour.
	withoutCruiseKubeCost := (snapshot.Data.CPU.WorkloadRequested/reqAllocRatioCPU)*p.CPUPerCorePerHour + (requestedMemGB/reqAllocRatioMem)*p.MemPerGBPerHour
	// Cost at recommended resources (With CruiseKube), per hour.
	withCruiseKubeCost := (snapshot.Data.CPU.RecommendedRequested/reqAllocRatioCPU)*p.CPUPerCorePerHour + (recommendedMemGB/reqAllocRatioMem)*p.MemPerGBPerHour
	return currentCost, withoutCruiseKubeCost, withCruiseKubeCost
}

func addTimelinePoint(out *[]types.HistoricalTimelineItem, legend, color string, threshold float64, timestamp time.Time, value float64) {
	*out = append(*out, types.HistoricalTimelineItem{
		Legend: legend,
		Color:  color,
		Threshold: types.HistoricalTimelineThreshold{
			Value: threshold,
			Color: "#ef4444",
		},
		Data: types.HistoricalTimelinePoint{
			Timestamp: timestamp.UTC(),
			Value:     math.Round(value*1000) / 1000,
		},
	})
}

// GetOverviewHistoricalTimelineHandler returns historical CPU/memory/cost timeline data for a cluster.
// GET /api/v1/clusters/:clusterID/ui/overview/historical-timeline/:metric?startTime=<UTC timestamp>&endTime=<UTC timestamp>
func (deps HandlerDependencies) GetOverviewHistoricalTimelineHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	metric := types.HistoricalTimelineMetric(c.Param("metric"))
	if metric != types.HistoricalTimelineMetricCPU &&
		metric != types.HistoricalTimelineMetricMemory &&
		metric != types.HistoricalTimelineMetricCost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric, expected one of: cpu, memory, cost"})
		return
	}
	startTime, err := parseUTCTimestamp(c.Query("startTime"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid startTime"})
		return
	}
	endTime, err := parseUTCTimestamp(c.Query("endTime"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endTime"})
		return
	}
	if endTime.Before(startTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endTime must be greater than or equal to startTime"})
		return
	}
	if endTime.Sub(startTime) > 90*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "time range too large; max allowed is 90 days"})
		return
	}

	if deps.Storage == nil {
		logging.Errorf(ctx, "Storage not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not available"})
		return
	}

	snapshots, err := deps.Storage.GetSnapshotsInRange(clusterID, startTime, endTime)
	if err != nil {
		logging.Errorf(ctx, "Failed to get snapshots for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := types.HistoricalTimelineResponse{
		Data: make([]types.HistoricalTimelineItem, 0, len(snapshots)*5),
	}

	switch metric {
	case types.HistoricalTimelineMetricCPU:
		for _, snapshot := range snapshots {
			threshold := snapshot.Data.CPU.CurrentAllocatable
			addTimelinePoint(&response.Data, "Allocatable", "#2563eb", threshold, snapshot.CreatedAt, snapshot.Data.CPU.CurrentAllocatable)
			addTimelinePoint(&response.Data, "Requested", "#f59e0b", threshold, snapshot.CreatedAt, snapshot.Data.CPU.CurrentRequested)
			addTimelinePoint(&response.Data, "Original Requested", "#b45309", threshold, snapshot.CreatedAt, snapshot.Data.CPU.WorkloadRequested)
			addTimelinePoint(&response.Data, "Usage", "#16a34a", threshold, snapshot.CreatedAt, snapshot.Data.CPU.CurrentUtilized)
			addTimelinePoint(&response.Data, "Recommended", "#7c3aed", threshold, snapshot.CreatedAt, snapshot.Data.CPU.RecommendedRequested)
		}
	case types.HistoricalTimelineMetricMemory:
		for _, snapshot := range snapshots {
			threshold := snapshot.Data.Memory.CurrentAllocatable
			addTimelinePoint(&response.Data, "Allocatable", "#2563eb", threshold, snapshot.CreatedAt, snapshot.Data.Memory.CurrentAllocatable)
			addTimelinePoint(&response.Data, "Requested", "#f59e0b", threshold, snapshot.CreatedAt, snapshot.Data.Memory.CurrentRequested)
			addTimelinePoint(&response.Data, "Original Requested", "#b45309", threshold, snapshot.CreatedAt, snapshot.Data.Memory.WorkloadRequested)
			addTimelinePoint(&response.Data, "Usage", "#16a34a", threshold, snapshot.CreatedAt, snapshot.Data.Memory.CurrentUtilized)
			addTimelinePoint(&response.Data, "Recommended", "#7c3aed", threshold, snapshot.CreatedAt, snapshot.Data.Memory.RecommendedRequested)
		}
	case types.HistoricalTimelineMetricCost:
		pricing := deps.getEffectivePricing(ctx, clusterID)
		for _, snapshot := range snapshots {
			currentCost, withoutCruiseKubeCost, withCruiseKubeCost := buildCostFromSnapshot(pricing, snapshot)
			addTimelinePoint(&response.Data, "Hourly Cost Without CruiseKube", "#f59e0b", currentCost, snapshot.CreatedAt, withoutCruiseKubeCost)
			addTimelinePoint(&response.Data, "Hourly Cost", "#2563eb", currentCost, snapshot.CreatedAt, currentCost)
			addTimelinePoint(&response.Data, "Hourly Cost With CruiseKube", "#16a34a", currentCost, snapshot.CreatedAt, withCruiseKubeCost)
		}
	}

	c.JSON(http.StatusOK, response)
}
