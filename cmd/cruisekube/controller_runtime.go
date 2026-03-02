package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/truefoundry/cruisekube/pkg/adapters/database"
	"github.com/truefoundry/cruisekube/pkg/adapters/kube"
	"github.com/truefoundry/cruisekube/pkg/adapters/metricsProvider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/audit"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/middleware"
	"github.com/truefoundry/cruisekube/pkg/oom"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/server"
	"github.com/truefoundry/cruisekube/pkg/task"
)

type controllerRuntime struct {
	clusterManager cluster.Manager
	promClient     *prometheus.PrometheusProvider
	storageRepo    *storage.Storage
}

func startControllerRuntime(ctx context.Context, cfg *config.Config) {
	runtime := buildControllerRuntime(ctx, cfg)

	startControllerHTTPServer(ctx, cfg, runtime.clusterManager)
	startOOMWorkers(ctx, cfg, runtime.clusterManager, runtime.storageRepo)
	registerControllerTasks(ctx, cfg, runtime.clusterManager, runtime.promClient, runtime.storageRepo)

	if err := runtime.clusterManager.ScheduleAllTasks(); err != nil {
		logging.Fatalf(ctx, "Failed to schedule tasks: %v", err)
	}
}

func buildControllerRuntime(ctx context.Context, cfg *config.Config) controllerRuntime {
	storageRepo := initStorageRepo(ctx, cfg)
	clusterManager, promClient := buildClusterRuntime(ctx, cfg)

	return controllerRuntime{
		clusterManager: clusterManager,
		promClient:     promClient,
		storageRepo:    storageRepo,
	}
}

func initStorageRepo(ctx context.Context, cfg *config.Config) *storage.Storage {
	databaseAdapter, err := database.NewDatabase(database.DatabaseConfig{
		Type:     cfg.DB.Type,
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		Database: cfg.DB.Database,
		Username: cfg.DB.Username,
		Password: cfg.DB.Password,
		SSLMode:  cfg.DB.SSLMode,
	})
	if err != nil {
		logging.Fatalf(ctx, "Failed to initialize database: %v", err)
	}
	logging.Infof(ctx, "Database initialized")

	storageRepo, err := storage.NewStorageRepo(databaseAdapter)
	if err != nil {
		logging.Fatalf(ctx, "Failed to initialize storage: %v", err)
	}
	logging.Infof(ctx, "Storage Repo initialized")

	storage.Stg = storageRepo
	audit.Recorder = audit.NewAudit(ctx, databaseAdapter, audit.Options{})

	return storageRepo
}

func buildClusterRuntime(ctx context.Context, cfg *config.Config) (cluster.Manager, *prometheus.PrometheusProvider) {
	switch cfg.ControllerMode {
	case config.ClusterModeLocal:
		return buildLocalClusterRuntime(ctx, cfg)
	case config.ClusterModeInCluster:
		return buildInClusterRuntime(ctx, cfg)
	default:
		logging.Fatalf(ctx, "Invalid controller mode: %s", cfg.ControllerMode)
		return nil, nil
	}
}

func buildLocalClusterRuntime(ctx context.Context, cfg *config.Config) (cluster.Manager, *prometheus.PrometheusProvider) {
	logging.Infof(ctx, "Local cluster mode")
	clusterCtx := contextutils.WithCluster(ctx, "local")

	kubeconfigPath := cfg.Dependencies.Local.KubeconfigPath
	if kubeconfigPath == "" {
		if home := homeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	kubeClient, err := kube.NewKubeClient(clusterCtx, kubeconfigPath)
	if err != nil {
		logging.Fatalf(ctx, "Failed to create kube client: %v", err)
	}

	dynamicClient, err := kube.NewDynamicClient(clusterCtx, kubeconfigPath)
	if err != nil {
		logging.Fatalf(ctx, "Failed to create dynamic client: %v", err)
	}

	promClient, err := prometheus.NewPrometheusProvider(clusterCtx, prometheus.GetPrometheusClientConfig(cfg.Dependencies.Local.PrometheusURL))
	if err != nil {
		logging.Fatalf(ctx, "Failed to create prometheus client: %v", err)
	}

	clusterManager := cluster.NewSingleClusterManager(clusterCtx, kubeClient, dynamicClient, promClient.GetClient())
	return clusterManager, promClient
}

func buildInClusterRuntime(ctx context.Context, cfg *config.Config) (cluster.Manager, *prometheus.PrometheusProvider) {
	logging.Infof(ctx, "In-cluster mode")
	clusterCtx := contextutils.WithCluster(ctx, "in-cluster")

	kubeClient, err := kube.NewKubeClient(clusterCtx, "")
	if err != nil {
		logging.Fatalf(ctx, "Failed to create kube client: %v", err)
	}

	dynamicClient, err := kube.NewDynamicClient(clusterCtx, "")
	if err != nil {
		logging.Fatalf(ctx, "Failed to create dynamic client: %v", err)
	}

	promClient, err := prometheus.NewPrometheusProvider(clusterCtx, prometheus.GetPrometheusClientConfig(cfg.Dependencies.InCluster.PrometheusURL))
	if err != nil {
		logging.Fatalf(ctx, "Failed to create prometheus client: %v", err)
	}

	clusterManager := cluster.NewSingleClusterManager(clusterCtx, kubeClient, dynamicClient, promClient.GetClient())
	return clusterManager, promClient
}

func startControllerHTTPServer(ctx context.Context, cfg *config.Config, clusterManager cluster.Manager) {
	engine := server.SetupServerEngine(
		clusterManager,
		middleware.AuthAPI(),
		middleware.AuthWebhook(),
		middleware.EnsureClusterExists(),
		cfg.Server.EnableDevAPIs,
		middleware.Common(clusterManager, cfg)...,
	)

	serverPort := cfg.Server.Port
	go func() {
		if err := engine.Run(":" + serverPort); err != nil {
			logging.Fatalf(ctx, "HTTP server failed: %v", err)
		}
	}()
}

func startOOMWorkers(ctx context.Context, cfg *config.Config, clusterManager cluster.Manager, storageRepo *storage.Storage) {
	for clusterID, clusterClients := range clusterManager.GetAllClusters() {
		oomObserver := oom.NewObserver(clusterClients.KubeClient)
		oomProcessor := oom.NewProcessor(storageRepo, clusterClients.KubeClient, clusterID, cfg)

		namespace := cfg.Controller.TargetNamespace

		if err := oomObserver.Start(ctx, clusterClients.KubeClient, namespace); err != nil {
			logging.Errorf(ctx, "Failed to start OOM observer for cluster %s: %v", clusterID, err)
		} else {
			logging.Infof(ctx, "OOM observer started for cluster %s", clusterID)
			oomProcessor.Start(ctx, oomObserver)
			logging.Infof(ctx, "OOM processor started for cluster %s", clusterID)
		}
	}
}

func registerControllerTasks(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	promClient *prometheus.PrometheusProvider,
	storageRepo *storage.Storage,
) {
	for clusterID, clusterClients := range clusterManager.GetAllClusters() {
		registerCreateStatsTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient, storageRepo)
		registerModifyEqualCPUResourcesTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient)
		registerApplyRecommendationTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient, storageRepo)
		registerFetchMetricsTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient, storageRepo)
		registerNodeLoadMonitoringTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient)
		registerCleanupOOMEventsTask(ctx, cfg, clusterManager, clusterID, storageRepo)
		registerDisruptionForceTask(ctx, cfg, clusterManager, clusterClients, clusterID, storageRepo)
	}
}

func registerCreateStatsTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterClients *cluster.ClusterClients,
	clusterID string,
	promClient *prometheus.PrometheusProvider,
	storageRepo *storage.Storage,
) {
	createStatsTaskConfig := cfg.GetTaskConfig(config.CreateStatsKey)

	clusterManager.AddTask(task.NewCreateStatsTask(
		ctx,
		clusterClients.KubeClient,
		clusterClients.DynamicClient,
		promClient,
		storageRepo,
		&task.CreateStatsTaskConfig{
			Name:                       clusterID + "_" + config.CreateStatsKey,
			Enabled:                    createStatsTaskConfig.Enabled,
			Schedule:                   createStatsTaskConfig.Schedule,
			ClusterID:                  clusterID,
			TargetClusterID:            cfg.Controller.TargetClusterID,
			TargetNamespace:            cfg.Controller.TargetNamespace,
			RecentStatsLookbackMinutes: 1,
			TimeStepSize:               5 * time.Minute,
			MLLookbackWindow:           1 * time.Hour,
		},
		createStatsTaskConfig,
	))
}

func registerModifyEqualCPUResourcesTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterClients *cluster.ClusterClients,
	clusterID string,
	promClient *prometheus.PrometheusProvider,
) {
	modifyEqualCPUResourcesTaskConfig := cfg.GetTaskConfig(config.ModifyEqualCPUResourcesKey)

	clusterManager.AddTask(task.NewModifyEqualCPUResourcesTask(
		ctx,
		clusterClients.KubeClient,
		clusterClients.DynamicClient,
		promClient,
		&task.ModifyEqualCPUResourcesTaskConfig{
			Name:                     clusterID + "_" + config.ModifyEqualCPUResourcesKey,
			Enabled:                  modifyEqualCPUResourcesTaskConfig.Enabled,
			Schedule:                 modifyEqualCPUResourcesTaskConfig.Schedule,
			ClusterID:                clusterID,
			IsClusterWriteAuthorized: cfg.IsClusterWriteAuthorized(clusterID),
		},
	))
}

func registerApplyRecommendationTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterClients *cluster.ClusterClients,
	clusterID string,
	promClient *prometheus.PrometheusProvider,
	storageRepo *storage.Storage,
) {
	applyRecommendationTaskConfig := cfg.GetTaskConfig(config.ApplyRecommendationKey)

	clusterManager.AddTask(task.NewApplyRecommendationTask(
		ctx,
		clusterClients.KubeClient,
		clusterClients.DynamicClient,
		promClient,
		storageRepo,
		&task.ApplyRecommendationTaskConfig{
			Name:                     clusterID + "_" + config.ApplyRecommendationKey,
			Enabled:                  applyRecommendationTaskConfig.Enabled,
			Schedule:                 applyRecommendationTaskConfig.Schedule,
			ClusterID:                clusterID,
			TargetClusterID:          cfg.Controller.TargetClusterID,
			TargetNamespace:          cfg.Controller.TargetNamespace,
			IsClusterWriteAuthorized: cfg.IsClusterWriteAuthorized(clusterID),
			BasicAuth:                cfg.Server.BasicAuth,
			RecommendationSettings:   cfg.RecommendationSettings,
		},
		applyRecommendationTaskConfig,
	))
}

func registerFetchMetricsTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterClients *cluster.ClusterClients,
	clusterID string,
	promClient *prometheus.PrometheusProvider,
	storageRepo *storage.Storage,
) {
	fetchMetricsTaskConfig := cfg.GetTaskConfig(config.FetchMetricsKey)

	clusterManager.AddTask(task.NewFetchMetricsTask(
		ctx,
		clusterClients.KubeClient,
		clusterClients.DynamicClient,
		promClient,
		storageRepo,
		&task.FetchMetricsTaskConfig{
			Name:      clusterID + "_" + config.FetchMetricsKey,
			Enabled:   fetchMetricsTaskConfig.Enabled,
			Schedule:  fetchMetricsTaskConfig.Schedule,
			ClusterID: clusterID,
		},
	))
}

func registerNodeLoadMonitoringTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterClients *cluster.ClusterClients,
	clusterID string,
	promClient *prometheus.PrometheusProvider,
) {
	nodeLoadMonitoringTaskConfig := cfg.GetTaskConfig(config.NodeLoadMonitoringKey)

	clusterManager.AddTask(task.NewNodeLoadMonitoringTask(
		ctx,
		clusterClients.KubeClient,
		clusterClients.DynamicClient,
		promClient,
		&task.NodeLoadMonitoringTaskConfig{
			Name:                     clusterID + "_" + config.NodeLoadMonitoringKey,
			Enabled:                  nodeLoadMonitoringTaskConfig.Enabled,
			Schedule:                 nodeLoadMonitoringTaskConfig.Schedule,
			ClusterID:                clusterID,
			IsClusterWriteAuthorized: cfg.IsClusterWriteAuthorized(clusterID),
		},
	))
}

func registerCleanupOOMEventsTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterID string,
	storageRepo *storage.Storage,
) {
	cleanupOOMEventsTaskConfig := cfg.GetTaskConfig(config.CleanupOOMEventsKey)

	clusterManager.AddTask(task.NewCleanupOOMEventsTask(
		ctx,
		storageRepo,
		&task.CleanupOOMEventsTaskConfig{
			Name:      clusterID + "_" + config.CleanupOOMEventsKey,
			Enabled:   cleanupOOMEventsTaskConfig.Enabled,
			Schedule:  cleanupOOMEventsTaskConfig.Schedule,
			ClusterID: clusterID,
		},
		cleanupOOMEventsTaskConfig,
	))
}

func registerDisruptionForceTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterClients *cluster.ClusterClients,
	clusterID string,
	storageRepo *storage.Storage,
) {
	disruptionForceTaskConfig := cfg.GetTaskConfig(config.DisruptionForceKey)

	clusterManager.AddTask(task.NewDisruptionForceTask(
		ctx,
		clusterClients.KubeClient,
		storageRepo,
		&task.DisruptionForceTaskConfig{
			Name:                     clusterID + "_" + config.DisruptionForceKey,
			Enabled:                  disruptionForceTaskConfig.Enabled,
			Schedule:                 disruptionForceTaskConfig.Schedule,
			ClusterID:                clusterID,
			IsClusterWriteAuthorized: cfg.IsClusterWriteAuthorized(clusterID),
		},
	))
}
