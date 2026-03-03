package main

import (
	"context"
	"fmt"
	"net/http"
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

func startControllerRuntime(runtimeManager *runtimeManager, cfg *config.Config) error {
	runtime, err := buildControllerRuntime(runtimeManager, cfg)
	if err != nil {
		return err
	}

	startControllerHTTPServer(runtimeManager, cfg, runtime.clusterManager)
	startOOMWorkers(runtimeManager.ctx, cfg, runtime.clusterManager, runtime.storageRepo)
	registerControllerTasks(runtimeManager.ctx, cfg, runtime.clusterManager, runtime.promClient, runtime.storageRepo)
	runtimeManager.Go("controller scheduler", func(context.Context) error {
		if err := runtime.clusterManager.ScheduleAllTasks(); err != nil {
			return fmt.Errorf("failed to schedule tasks: %w", err)
		}
		return nil
	})

	return nil
}

func buildControllerRuntime(runtimeManager *runtimeManager, cfg *config.Config) (controllerRuntime, error) {
	storageRepo, err := initStorageRepo(runtimeManager, cfg)
	if err != nil {
		return controllerRuntime{}, err
	}
	clusterManager, promClient, err := buildClusterRuntime(runtimeManager.ctx, cfg)
	if err != nil {
		return controllerRuntime{}, err
	}

	return controllerRuntime{
		clusterManager: clusterManager,
		promClient:     promClient,
		storageRepo:    storageRepo,
	}, nil
}

func initStorageRepo(runtimeManager *runtimeManager, cfg *config.Config) (*storage.Storage, error) {
	ctx := runtimeManager.ctx
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
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	logging.Infof(ctx, "Database initialized")
	runtimeManager.AddCleanup(func(context.Context) error {
		return databaseAdapter.Close()
	})

	storageRepo, err := storage.NewStorageRepo(databaseAdapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}
	logging.Infof(ctx, "Storage Repo initialized")

	storage.Stg = storageRepo
	recorder := audit.NewAudit(ctx, databaseAdapter, audit.Options{})
	audit.Recorder = recorder
	runtimeManager.AddCleanup(func(context.Context) error {
		recorder.Close()
		return nil
	})

	return storageRepo, nil
}

func buildClusterRuntime(ctx context.Context, cfg *config.Config) (cluster.Manager, *prometheus.PrometheusProvider, error) {
	switch cfg.ControllerMode {
	case config.ClusterModeLocal:
		return buildLocalClusterRuntime(ctx, cfg)
	case config.ClusterModeInCluster:
		return buildInClusterRuntime(ctx, cfg)
	default:
		return nil, nil, fmt.Errorf("invalid controller mode: %s", cfg.ControllerMode)
	}
}

func buildLocalClusterRuntime(ctx context.Context, cfg *config.Config) (cluster.Manager, *prometheus.PrometheusProvider, error) {
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
		return nil, nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	dynamicClient, err := kube.NewDynamicClient(clusterCtx, kubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	promClient, err := prometheus.NewPrometheusProvider(clusterCtx, prometheus.GetPrometheusClientConfig(cfg.Dependencies.Local.PrometheusURL))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	clusterManager := cluster.NewSingleClusterManager(clusterCtx, kubeClient, dynamicClient, promClient.GetClient())
	return clusterManager, promClient, nil
}

func buildInClusterRuntime(ctx context.Context, cfg *config.Config) (cluster.Manager, *prometheus.PrometheusProvider, error) {
	logging.Infof(ctx, "In-cluster mode")
	clusterCtx := contextutils.WithCluster(ctx, "in-cluster")

	kubeClient, err := kube.NewKubeClient(clusterCtx, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	dynamicClient, err := kube.NewDynamicClient(clusterCtx, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	promClient, err := prometheus.NewPrometheusProvider(clusterCtx, prometheus.GetPrometheusClientConfig(cfg.Dependencies.InCluster.PrometheusURL))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	clusterManager := cluster.NewSingleClusterManager(clusterCtx, kubeClient, dynamicClient, promClient.GetClient())
	return clusterManager, promClient, nil
}

func startControllerHTTPServer(runtimeManager *runtimeManager, cfg *config.Config, clusterManager cluster.Manager) {
	engine := server.SetupServerEngine(
		clusterManager,
		middleware.AuthAPI(),
		middleware.AuthWebhook(),
		middleware.EnsureClusterExists(),
		cfg.Server.EnableDevAPIs,
		middleware.Common(clusterManager, cfg)...,
	)

	startHTTPServer(runtimeManager, "controller HTTP server", "Starting HTTP server on :"+cfg.Server.Port, &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}, func(server *http.Server) error {
		return server.ListenAndServe()
	})
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
