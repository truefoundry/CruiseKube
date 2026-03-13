package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"

	"github.com/gin-gonic/gin"
)

func (deps HandlerDependencies) ListWorkloadsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	logging.Infof(ctx, "Listing workloads for cluster %s", clusterID)

	workloadsInCluster, err := deps.Storage.GetWorkloadsInCluster(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get workloads in cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get workloads in cluster %s: %v", clusterID, err),
		})
		return
	}

	var workloads = make([]types.WorkloadOverrideInfo, 0)
	for _, workload := range workloadsInCluster {
		overrides := workload.OverridesWithDefaults()
		stat := workload.Stat
		if stat == nil {
			continue
		}
		if stat.IsGPUWorkload() {
			continue
		}

		output := types.WorkloadOverrideInfo{
			WorkloadID: workload.WorkloadID,
			Name:       stat.Name,
			Namespace:  stat.Namespace,
			Kind:       stat.Kind,
			Overrides:  overrides,
		}
		workloads = append(workloads, output)
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
