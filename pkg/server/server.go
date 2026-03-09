package server

import (
	"github.com/truefoundry/cruisekube/pkg/handlers"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func SetupServerEngine(handlerDeps handlers.HandlerDependencies, authAPI gin.HandlerFunc, authWebhook gin.HandlerFunc, ensureClusterExists gin.HandlerFunc, enableDevAPIs bool, middleware ...gin.HandlerFunc) *gin.Engine {
	r := gin.Default()
	r.Use(otelgin.Middleware("cruisekube-api"))
	r.Use(middleware...)

	r.GET("/health", handlers.HandleHealth)

	apiV1Group := r.Group("/api/v1")
	{
		apiV1Group.GET("/", handlers.HandleRoot)
		apiV1Group.GET("/clusters", authAPI, handlerDeps.HandleListClusters)
	}

	clusterGroup := apiV1Group.Group("/clusters/:clusterID", authAPI, ensureClusterExists)
	{
		clusterGroup.GET("/stats", handlerDeps.HandleClusterStats)
		clusterGroup.GET("/config", handlerDeps.GetConfigHandler)
		clusterGroup.POST("/killswitch", handlerDeps.KillswitchHandler)
		clusterGroup.GET("/workloads", handlerDeps.ListWorkloadsHandler)
		clusterGroup.GET("/workloads/summary", handlerDeps.WorkloadSummaryHandler)
		clusterGroup.GET("/workloads/:namespace/:workloadName/detail", handlerDeps.HandleWorkloadDetail)
		clusterGroup.POST("/workloads/:workloadID/overrides", handlerDeps.UpdateWorkloadOverridesHandler)
		clusterGroup.POST("/workloads/batch-overrides", handlerDeps.BatchUpdateWorkloadOverridesHandler)
		// Audit Events
		clusterGroup.GET("/audit-events", handlerDeps.GetAuditEventsHandler)
		clusterGroup.GET("/audit-events/:workloadID", handlerDeps.GetAuditEventsForWorkloadHandler)
		// UI Overview
		clusterGroup.GET("/ui/overview", handlerDeps.OverviewHandler)
		clusterGroup.GET("/ui/overview/historical-timeline/:metric", handlerDeps.GetOverviewHistoricalTimelineHandler)
		// Settings
		clusterGroup.GET("/settings", handlerDeps.GetSettingsHandler)
		clusterGroup.PUT("/settings", handlerDeps.UpdateSettingsHandler)
	}

	if enableDevAPIs {
		devGroup := apiV1Group.Group("/dev/clusters/:clusterID", authAPI, ensureClusterExists)
		{
			devGroup.POST("/tasks/:taskName/trigger", handlerDeps.HandleTaskTrigger)
		}
	}

	webhookGroup := apiV1Group.Group("/webhook/clusters/:clusterID", authWebhook, ensureClusterExists)
	{
		webhookGroup.POST("/mutate", handlerDeps.HandleMutatingPatch)
	}

	return r
}

func SetupWebhookServerEngine(handlerDeps handlers.HandlerDependencies, middleware ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(otelgin.Middleware("cruisekubeWebhook"))
	r.Use(middleware...)

	clusterGroup := r.Group("/clusters/:clusterID")
	{
		clusterGroup.POST("/webhook/mutate", handlerDeps.MutateHandler)
	}

	return r
}

func SetupMetricsServerEngine() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", handlers.HandleHealth)

	return r
}
