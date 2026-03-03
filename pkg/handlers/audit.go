package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
)

const defaultAuditMinutes = 60
const maxAuditMinutes = 43200 // 30 days

// parseMinutesParam parses the "minutes" query param; returns defaultAuditMinutes if missing or invalid, clamped to [1, maxAuditMinutes].
func parseMinutesParam(c *gin.Context) int {
	s := c.DefaultQuery("minutes", strconv.Itoa(defaultAuditMinutes))
	m, err := strconv.Atoi(s)
	if err != nil || m < 1 {
		return defaultAuditMinutes
	}
	if m > maxAuditMinutes {
		return maxAuditMinutes
	}
	return m
}

// GetAuditEventsHandler returns all audit events for the cluster from the last x minutes.
// GET /api/v1/clusters/:clusterID/audit-events?minutes=60
func GetAuditEventsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	minutes := parseMinutesParam(c)

	if storage.Stg == nil {
		logging.Errorf(ctx, "Storage not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not available"})
		return
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	events, err := storage.Stg.GetAuditEvents(clusterID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get audit events for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}

// GetAuditEventsForWorkloadHandler returns audit events for a specific workload from the last x minutes.
// GET /api/v1/clusters/:clusterID/audit-events/:workloadID?minutes=60
func GetAuditEventsForWorkloadHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	workloadID := c.Param("workloadID")
	minutes := parseMinutesParam(c)

	if storage.Stg == nil {
		logging.Errorf(ctx, "Storage not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not available"})
		return
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	events, err := storage.Stg.GetAuditEventsForWorkload(clusterID, workloadID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get audit events for workload %s in cluster %s: %v", workloadID, clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}
