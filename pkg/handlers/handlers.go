package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
	"go.opentelemetry.io/otel/attribute"

	oteltrace "go.opentelemetry.io/otel/trace"
)

func HandleRoot(c *gin.Context) {
	c.Data(http.StatusOK, "application/json",
		[]byte(`{
			"message": "cruisekube API Server",
			"endpoints": {
				"/clusters": "Lists all available clusters",
				"/clusters/{clusterId}/stats": "Serves stats file for specific cluster",
				"/clusters/{clusterId}/killswitch": "Deletes MutatingWebhookConfiguration objects and kills pods with resource differences (POST only)",
				"/clusters/{clusterId}/webhook/mutate": "Mutating admission webhook for pod resource adjustment",
				"/dev/clusters/{clusterId}/tasks/{taskName}/trigger": "Manually triggers a specific task (POST)",
				"/health": "Health check endpoint"
			}
		}`),
	)
}

func HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (deps HandlerDependencies) HandleListClusters(c *gin.Context) {
	ctx := c.Request.Context()
	mgr := deps.ClusterManager

	logging.Infof(ctx, "Serving cluster list to %s", c.ClientIP())

	clusterIDs := mgr.GetClusterIDs()

	clusters := make([]map[string]any, 0, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		statsExists, err := deps.Storage.ClusterStatsExists(clusterID)
		if err != nil {
			logging.Errorf(ctx, "Failed to check if cluster stats exists for %s: %v", clusterID, err)
			continue
		}
		clusters = append(clusters, map[string]any{
			"id":              clusterID,
			"name":            clusterID,
			"stats_available": statsExists,
		})
	}

	response := map[string]any{
		"clusters":     clusters,
		"count":        len(clusters),
		"cluster_mode": mgr.GetClusterMode(),
	}

	c.JSON(http.StatusOK, response)
}

func (deps HandlerDependencies) HandleClusterStats(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("cluster", clusterID))

	logging.Infof(ctx, "Serving stats for cluster %s to %s", clusterID, c.ClientIP())
	c.Header("Content-Type", "application/json")
	var statsResponse types.StatsResponse
	if err := deps.Storage.ReadClusterStats(clusterID, &statsResponse); err != nil {
		logging.Errorf(ctx, "Failed to read cluster stats for %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read cluster stats for %s: %v", clusterID, err),
		})
		return
	}
	// Do not send GPU workloads to frontend
	filtered := make([]types.WorkloadStat, 0, len(statsResponse.Stats))
	for i := range statsResponse.Stats {
		if !statsResponse.Stats[i].IsGPUWorkload() {
			filtered = append(filtered, statsResponse.Stats[i])
		}
	}
	statsResponse.Stats = filtered
	c.JSON(http.StatusOK, statsResponse)
}
