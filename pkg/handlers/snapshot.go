package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
)

const defaultSnapshotMinutes = 60
const maxSnapshotMinutes = 43200 // 30 days

func parseSnapshotMinutesParam(c *gin.Context) int {
	s := c.DefaultQuery("minutes", strconv.Itoa(defaultSnapshotMinutes))
	m, err := strconv.Atoi(s)
	if err != nil || m < 1 {
		return defaultSnapshotMinutes
	}
	if m > maxSnapshotMinutes {
		return maxSnapshotMinutes
	}
	return m
}

// GetSnapshotsHandler returns all snapshots for the cluster from the last x minutes.
// GET /api/v1/clusters/:clusterID/snapshots?minutes=60
func GetSnapshotsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	minutes := parseSnapshotMinutesParam(c)

	if storage.Stg == nil {
		logging.Errorf(ctx, "Storage not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not available"})
		return
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	snapshots, err := storage.Stg.GetSnapshots(clusterID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get snapshots for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"snapshots": snapshots})
}
