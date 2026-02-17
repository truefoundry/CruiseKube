package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/types"

	"github.com/gin-gonic/gin"
)

func ListWorkloadsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	logging.Infof(ctx, "Listing workloads for cluster %s", clusterID)
	// Only return workloads with data updated in the last 24 hours
	since := time.Now().Add(-StatsAPIDataLookbackWindow)
	stats, err := storage.Stg.GetAllStatsForClusterUpdatedSince(clusterID, since)
	if err != nil {
		logging.Errorf(ctx, "Failed to get stats for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get workloads for cluster %s: %v", clusterID, err),
		})
		return
	}

	var workloads = make([]types.WorkloadOverrideInfo, 0)
	for _, stat := range stats {
		if stat.IsGPUWorkload() {
			continue
		}
		workloadColumnId := strings.ReplaceAll(stat.WorkloadIdentifier, "/", ":")
		overrides, err := storage.Stg.GetWorkloadOverrides(clusterID, workloadColumnId)
		if err != nil {
			logging.Errorf(ctx, "Failed to get overrides for workload %s: %v", stat.WorkloadIdentifier, err)
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

		workloadExternalId := strings.ReplaceAll(stat.WorkloadIdentifier, "/", ":")
		workload := types.WorkloadOverrideInfo{
			WorkloadID: workloadExternalId,
			Name:       stat.Name,
			Namespace:  stat.Namespace,
			Kind:       stat.Kind,
			Overrides: &types.WorkloadOverridesEffective{
				EvictionRanking: evictionRanking,
				Enabled:         enabled,
			},
		}
		workloads = append(workloads, workload)
	}

	c.Header("Content-Type", "application/json")
	if err := json.NewEncoder(c.Writer).Encode(workloads); err != nil {
		logging.Errorf(ctx, "Failed to encode workloads: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to encode workloads: %v", err),
		})
		return
	}
}
