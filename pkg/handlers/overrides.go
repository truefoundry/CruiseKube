package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"

	"github.com/gin-gonic/gin"
)

func GetWorkloadOverridesHandler(c *gin.Context) {
	clusterID := c.Param("clusterID")
	workloadID := c.Param("workloadID")
	overrides, err := storage.Stg.GetWorkloadOverrides(clusterID, workloadID)
	if err != nil {
		logging.Errorf(c.Request.Context(), "Failed to get workload overrides: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get workload overrides: %v", err),
		})
		return
	}

	if overrides == nil {
		overrides = &types.Overrides{
			Enabled:         utils.PtrTo(true),
			EvictionRanking: utils.PtrTo(types.EvictionRankingHigh),
		}
	}

	c.Header("Content-Type", "application/json")
	if err := json.NewEncoder(c.Writer).Encode(overrides); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to encode response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to encode response: %v", err),
		})
		return
	}
}

func UpdateWorkloadOverridesHandler(c *gin.Context) {
	clusterID := c.Param("clusterID")
	var req types.UpdateWorkloadOverridesRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to decode request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}
	if req.WorkloadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "workload_id is required in request body",
		})
		return
	}
	if req.Overrides == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "overrides is required in request body",
		})
		return
	}

	if err := storage.Stg.UpdateWorkloadOverrides(clusterID, req.WorkloadID, req.Overrides); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to update workload overrides: %v", err)
		if strings.Contains(err.Error(), "workload not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Workload not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update workload overrides",
			})
		}
		return
	}

	c.Header("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"message":     "Workload overrides updated successfully",
		"cluster_id":  clusterID,
		"workload_id": req.WorkloadID,
		"overrides":   req.Overrides,
	}

	if err := json.NewEncoder(c.Writer).Encode(response); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to encode response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to encode response: %v", err),
		})
		return
	}

	logging.Infof(c.Request.Context(), "Successfully updated overrides for workload %s in cluster %s", req.WorkloadID, clusterID)
}
