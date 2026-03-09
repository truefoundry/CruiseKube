package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func (deps HandlerDependencies) UpdateWorkloadOverridesHandler(c *gin.Context) {
	clusterID := c.Param("clusterID")
	workloadID := c.Param("workloadID")
	var overrides *types.Overrides
	if err := json.NewDecoder(c.Request.Body).Decode(&overrides); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to decode request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	if err := deps.Storage.UpdateWorkloadOverrides(clusterID, workloadID, overrides); err != nil {
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
		"workload_id": workloadID,
		"overrides":   overrides,
	}

	if err := json.NewEncoder(c.Writer).Encode(response); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to encode response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to encode response: %v", err),
		})
		return
	}

	logging.Infof(c.Request.Context(), "Successfully updated overrides for workload %s in cluster %s", workloadID, clusterID)
}

type batchOverridesRequest struct {
	WorkloadIDs []string         `json:"workload_ids"`
	Overrides   *types.Overrides `json:"overrides"`
}

type batchOverridesResponse struct {
	Message   string   `json:"message"`
	ClusterID string   `json:"cluster_id"`
	Updated   []string `json:"updated"`
	NotFound  []string `json:"not_found"`
}

func (deps HandlerDependencies) BatchUpdateWorkloadOverridesHandler(c *gin.Context) {
	clusterID := c.Param("clusterID")

	var req batchOverridesRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logging.Errorf(c.Request.Context(), "Failed to decode request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	if len(req.WorkloadIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workload_ids must not be empty"})
		return
	}

	if req.Overrides == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overrides must not be null"})
		return
	}

	updated, notFound, err := deps.Storage.BatchUpdateWorkloadOverrides(clusterID, req.WorkloadIDs, req.Overrides)
	if err != nil {
		logging.Errorf(c.Request.Context(), "Failed to batch update workload overrides: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to batch update workload overrides",
		})
		return
	}

	if updated == nil {
		updated = []string{}
	}
	if notFound == nil {
		notFound = []string{}
	}

	logging.Infof(c.Request.Context(), "Batch overrides update in cluster %s: %d updated, %d not found", clusterID, len(updated), len(notFound))

	c.JSON(http.StatusOK, batchOverridesResponse{
		Message:   "Batch update completed",
		ClusterID: clusterID,
		Updated:   updated,
		NotFound:  notFound,
	})
}
