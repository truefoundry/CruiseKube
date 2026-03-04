package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/adapters/metricsProvider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
				"/clusters/{clusterId}/prometheus-proxy": "Proxies requests to cluster's Prometheus instance",
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

func (deps HandlerDependencies) HandlePrometheusProxy(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("cluster.id", clusterID))

	logging.Infof(ctx, "Proxying prometheus request for cluster %s from %s", clusterID, c.ClientIP())

	connInfo, err := deps.ClusterManager.GetPrometheusConnectionInfo(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get prometheus connection info for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get prometheus connection info for cluster %s: %v", clusterID, err),
		})
		return
	}

	targetURL, err := url.Parse(connInfo.URL)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse prometheus URL for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to parse prometheus URL for cluster %s: %v", clusterID, err),
		})
		return
	}

	targetURL.Path = strings.Join([]string{targetURL.Path, strings.TrimPrefix(c.Request.URL.Path, fmt.Sprintf("/api/v1/clusters/%s/prometheus-proxy", clusterID))}, "")
	targetURL.RawQuery = c.Request.URL.RawQuery

	proxyReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL.String(), c.Request.Body)
	if err != nil {
		logging.Errorf(ctx, "Failed to create proxy request for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create proxy request for cluster %s: %v", clusterID, err),
		})
		return
	}

	for header, values := range c.Request.Header {
		for _, value := range values {
			proxyReq.Header.Add(header, value)
		}
	}

	proxyReq.Header.Set("Authorization", "Bearer "+connInfo.BearerToken)

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	resp, err := client.Do(proxyReq)
	if err != nil {
		logging.Errorf(ctx, "Failed to proxy request to prometheus for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": fmt.Sprintf("Failed to proxy request to prometheus for cluster %s: %v", clusterID, err),
		})
		return
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			logging.Errorf(ctx, "Failed to close response body: %v", err)
		}
	}()

	for header, values := range resp.Header {
		for _, value := range values {
			c.Header(header, value)
		}
	}

	c.Status(resp.StatusCode)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		logging.Errorf(ctx, "Failed to copy response body for cluster %s: %v", clusterID, err)
	}
}

func (deps HandlerDependencies) HandlePrometheusQuery(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("cluster.id", clusterID))

	query := c.Query("query")
	if query == "" {
		logging.Errorf(ctx, "Missing query parameter for cluster %s", clusterID)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":    "error",
			"errorType": "bad_data",
			"error":     "Missing query parameter",
		})
		return
	}

	logging.Infof(ctx, "Executing Prometheus query for cluster %s from %s", clusterID, c.ClientIP())

	clients, err := deps.ClusterManager.GetClusterClients(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get cluster clients for %s: %v", clusterID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"status":    "error",
			"errorType": "not_found",
			"error":     fmt.Sprintf("Cluster %s not found: %v", clusterID, err),
		})
		return
	}

	if clients.PrometheusClient == nil {
		logging.Errorf(ctx, "Prometheus client not available for cluster %s", clusterID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":    "error",
			"errorType": "internal",
			"error":     fmt.Sprintf("Prometheus client not available for cluster %s", clusterID),
		})
		return
	}

	result, warnings, err := clients.PrometheusClient.Query(ctx, query, time.Now())
	if err != nil {
		logging.Errorf(ctx, "Failed to execute Prometheus query for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":    "error",
			"errorType": "execution",
			"error":     fmt.Sprintf("Query execution failed: %v", err),
		})
		return
	}

	warningsList := make([]string, len(warnings))
	copy(warningsList, warnings)

	response, err := prometheus.ConvertModelValueToPrometheusJSON(result, warningsList)
	if err != nil {
		logging.Errorf(ctx, "Failed to convert Prometheus result for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":    "error",
			"errorType": "internal",
			"error":     fmt.Sprintf("Failed to convert result: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
