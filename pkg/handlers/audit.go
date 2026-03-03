package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
)

const defaultAuditMinutes = 60
const maxAuditMinutes = 43200 // 30 days

// parseMinutesParam parses the "minutes" query param. Returns an error if missing, invalid, or out of range [1, maxAuditMinutes].
func parseMinutesParam(c *gin.Context) (int, error) {
	s := c.DefaultQuery("minutes", strconv.Itoa(defaultAuditMinutes))
	m, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("minutes must be a valid integer, got %q", s)
	}
	if m < 1 {
		return 0, fmt.Errorf("minutes must be at least 1, got %d", m)
	}
	if m > maxAuditMinutes {
		return 0, fmt.Errorf("minutes must be at most %d, got %d", maxAuditMinutes, m)
	}
	return m, nil
}

// GetAuditEventsHandler returns all audit events for the cluster from the last x minutes.
// GET /api/v1/clusters/:clusterID/audit-events?minutes=60
func (deps HandlerDependencies) GetAuditEventsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	minutes, err := parseMinutesParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if deps.Storage == nil {
		logging.Errorf(ctx, "Storage not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not available"})
		return
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	events, err := deps.Storage.GetAuditEvents(clusterID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get audit events for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}

// GetAuditEventsForWorkloadHandler returns audit events for a specific workload from the last x minutes.
// GET /api/v1/clusters/:clusterID/audit-events/:workloadID?minutes=60
func (deps HandlerDependencies) GetAuditEventsForWorkloadHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	workloadID := c.Param("workloadID")
	minutes, err := parseMinutesParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if deps.Storage == nil {
		logging.Errorf(ctx, "Storage not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not available"})
		return
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	events, err := deps.Storage.GetAuditEventsForWorkload(clusterID, workloadID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get audit events for workload %s in cluster %s: %v", workloadID, clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}
