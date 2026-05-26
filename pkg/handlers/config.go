package handlers

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/redaction"
	"go.opentelemetry.io/otel/attribute"

	oteltrace "go.opentelemetry.io/otel/trace"
)

func (deps HandlerDependencies) GetConfigHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("cluster", clusterID))

	logging.Infof(ctx, "Getting metrics provider config for cluster %s", clusterID)

	cfg := deps.Config
	mgr := deps.ClusterManager

	providerConfig, err := cfg.ActiveMetricsProviderConfig()
	if err != nil {
		logging.Errorf(ctx, "Failed to resolve metrics provider config: %v", err)
		c.JSON(500, metricsProviderConfigResponse(config.MetricsProviderConfig{}, false, "", sanitizeProviderError(fmt.Sprintf("Failed to resolve metrics provider config: %v", err), "")))
		return
	}

	clients, err := mgr.GetClusterClients(clusterID)
	if err != nil {
		clusterClientError := sanitizeProviderError(fmt.Sprintf("Failed to get cluster clients: %v", err), providerConfig.BearerToken)
		logging.Errorf(ctx, "%s for %s", clusterClientError, clusterID)
		c.JSON(500, metricsProviderConfigResponse(providerConfig, false, "", clusterClientError))
		return
	}

	connected := false
	providerVersion := ""
	var connectionError string

	if clients.PrometheusClient != nil {
		_, _, err := clients.PrometheusClient.Query(ctx, "vector(1)", time.Now())
		if err != nil {
			connectionError = sanitizeProviderError(fmt.Sprintf("Connection test failed: %v", err), providerConfig.BearerToken)
			logging.Errorf(ctx, "Metrics provider connection test failed for cluster %s: %s", clusterID, connectionError)
		} else {
			connected = true
			logging.Infof(ctx, "Metrics provider connection test successful for cluster %s", clusterID)

			if providerConfig.Type == config.MetricsProviderTypePrometheus {
				if buildInfo, err := clients.PrometheusClient.Buildinfo(ctx); err != nil {
					logging.Warnf(ctx, "Prometheus build info unavailable for cluster %s: %v", clusterID, err)
				} else {
					providerVersion = buildInfo.Version
				}
			}
		}
	} else {
		connectionError = "Metrics provider client not available"
		logging.Errorf(ctx, "Metrics provider client not available for cluster %s", clusterID)
	}

	c.JSON(200, metricsProviderConfigResponse(providerConfig, connected, providerVersion, connectionError))
}

func metricsProviderConfigResponse(providerConfig config.MetricsProviderConfig, connected bool, providerVersion string, connectionError string) gin.H {
	response := gin.H{
		"url":       providerConfig.URL,
		"provider":  string(providerConfig.Type),
		"connected": connected,
		"version":   buildmetadata.Version,
	}
	if providerVersion != "" {
		response["providerVersion"] = providerVersion
	}
	if connectionError != "" {
		response["error"] = connectionError
	}
	return response
}

func sanitizeProviderError(message, bearerToken string) string {
	return redaction.String(message, bearerToken)
}
