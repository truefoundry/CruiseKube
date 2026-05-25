package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/truefoundry/cruisekube/pkg/adapters/database"
	"github.com/truefoundry/cruisekube/pkg/adapters/kube"
	"github.com/truefoundry/cruisekube/pkg/adapters/metricsprovider"
	"github.com/truefoundry/cruisekube/pkg/adapters/metricsprovider/prometheus"
	usageadapter "github.com/truefoundry/cruisekube/pkg/adapters/usagetelemetry"
	"github.com/truefoundry/cruisekube/pkg/audit"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/handlers"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/middleware"
	"github.com/truefoundry/cruisekube/pkg/oom"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/server"
	"github.com/truefoundry/cruisekube/pkg/task"
	"github.com/truefoundry/cruisekube/pkg/usageheartbeat"
)

type controllerRuntime struct {
	clusterManager cluster.Manager
	promClient     *prometheus.PrometheusProvider
	storageRepo    *storage.Storage
	auditRecorder  *audit.Audit
}

func startControllerRuntime(runtimeManager *runtimeManager, cfg *config.Config) error {
	runtime, err := buildControllerRuntime(runtimeManager, cfg)
	if err != nil {
		return err
	}

	runtimeManager.AddCleanup(func(ctx context.Context) error {
		runtime.clusterManager.StopScheduler(ctx)
		return nil
	})

	handlerDeps, err := handlers.NewHandlerDependencies(
		runtime.storageRepo,
		runtime.clusterManager,
		cfg,
		runtime.auditRecorder,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize handler dependencies: %w", err)
	}

	startControllerHTTPServer(runtimeManager, cfg, handlerDeps)
	startOOMWorkers(runtimeManager.ctx, cfg, runtime.clusterManager, runtime.storageRepo)
	registerControllerTasks(runtimeManager.ctx, cfg, runtime.clusterManager, runtime.promClient, runtime.storageRepo)
	if err := runtime.clusterManager.ScheduleAllTasks(runtimeManager.ctx); err != nil {
		return fmt.Errorf("failed to schedule tasks: %w", err)
	}

	startUsageTelemetryHeartbeat(runtimeManager, cfg, runtime.clusterManager)

	return nil
}

func startUsageTelemetryHeartbeat(runtimeManager *runtimeManager, cfg *config.Config, clusterManager cluster.Manager) {
	if !cfg.UsageTelemetry.Enabled {
		return
	}
	clients, err := clusterManager.GetClusterClients(cluster.SingleClusterID)
	if err != nil {
		logging.Errorf(runtimeManager.ctx, "usage telemetry: no cluster client: %v", err)
		return
	}
	reporter, err := usageadapter.NewReporter(cfg.UsageTelemetry.InstallID, cfg.UsageTelemetry.ProviderConfig, cfg.EffectiveUsageTelemetryProviderAPIKey())
	if err != nil {
		logging.Errorf(runtimeManager.ctx, "usage telemetry: reporter init failed: %v", err)
		return
	}
	runtimeManager.AddCleanup(func(context.Context) error {
		return reporter.Close()
	})
	runtimeManager.Go("usage telemetry heartbeat", func(ctx context.Context) error {
		return usageheartbeat.Start(ctx, clients.KubeClient, cfg, reporter)
	})
}

func buildControllerRuntime(runtimeManager *runtimeManager, cfg *config.Config) (controllerRuntime, error) {
	databaseAdapter, err := initDatabaseAdapter(runtimeManager, cfg)
	if err != nil {
		return controllerRuntime{}, err
	}

	storageRepo, err := initStorageRepo(runtimeManager.ctx, databaseAdapter)
	if err != nil {
		return controllerRuntime{}, err
	}

	auditRecorder := initAuditRecorder(runtimeManager, databaseAdapter)
	clusterManager, promClient, err := buildClusterRuntime(runtimeManager.ctx, cfg)
	if err != nil {
		return controllerRuntime{}, err
	}

	return controllerRuntime{
		clusterManager: clusterManager,
		promClient:     promClient,
		storageRepo:    storageRepo,
		auditRecorder:  auditRecorder,
	}, nil
}

func initDatabaseAdapter(runtimeManager *runtimeManager, cfg *config.Config) (ports.Database, error) {
	dbCfg := database.DatabaseConfig{
		Type:     cfg.DB.Type,
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		Database: cfg.DB.Database,
		Username: cfg.DB.Username,
		Password: cfg.DB.Password,
		SSLMode:  cfg.DB.SSLMode,
	}

	databaseAdapter, err := backoff.Retry(
		runtimeManager.ctx,
		func() (ports.Database, error) {
			return database.NewDatabase(dbCfg)
		},
		backoff.WithMaxElapsedTime(time.Minute),
		backoff.WithNotify(func(err error, d time.Duration) {
			logging.Infof(runtimeManager.ctx, "Failed to initialize database, retrying in %s: %v", d, err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	logging.Infof(runtimeManager.ctx, "Database initialized")
	runtimeManager.AddCleanup(func(context.Context) error {
		return databaseAdapter.Close()
	})

	return databaseAdapter, nil
}

func initStorageRepo(ctx context.Context, databaseAdapter ports.Database) (*storage.Storage, error) {
	storageRepo, err := storage.NewStorageRepo(databaseAdapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}
	logging.Infof(ctx, "Storage Repo initialized")

	// TODO: Remove global singleton assignments once all handlers are migrated to HandlerDependencies.
	storage.Stg = storageRepo
	return storageRepo, nil
}

func initAuditRecorder(runtimeManager *runtimeManager, databaseAdapter ports.Database) *audit.Audit {
	ctx := runtimeManager.ctx
	recorder := audit.NewAudit(ctx, databaseAdapter, audit.Options{})
	// TODO: Remove global singleton assignments once all handlers are migrated to HandlerDependencies.
	audit.Recorder = recorder
	runtimeManager.AddCleanup(func(context.Context) error {
		recorder.Close()
		return nil
	})

	return recorder
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

	metricsProvider, err := buildMetricsProvider(clusterCtx, cfg)
	if err != nil {
		return nil, nil, err
	}

	clusterManager := cluster.NewSingleClusterManager(clusterCtx, kubeClient, dynamicClient, metricsProvider.GetClient())
	return clusterManager, metricsProvider, nil
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

	metricsProvider, err := buildMetricsProvider(clusterCtx, cfg)
	if err != nil {
		return nil, nil, err
	}

	clusterManager := cluster.NewSingleClusterManager(clusterCtx, kubeClient, dynamicClient, metricsProvider.GetClient())
	return clusterManager, metricsProvider, nil
}

func buildMetricsProvider(ctx context.Context, cfg *config.Config) (*prometheus.PrometheusProvider, error) {
	metricsProviderConfig, err := cfg.ActiveMetricsProviderConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve active metrics provider config: %w", err)
	}

	metricsProvider, err := metricsprovider.NewProvider(ctx, metricsProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics provider: %w", err)
	}
	return metricsProvider, nil
}

func startControllerHTTPServer(runtimeManager *runtimeManager, cfg *config.Config, handlerDeps handlers.HandlerDependencies) {
	engine := server.SetupServerEngine(
		handlerDeps,
		middleware.AuthBasic(cfg.Server.Auth),
		middleware.AuthWebhook(),
		handlers.HandleLogin(cfg.Server.Auth),
		middleware.EnsureClusterExists(handlerDeps.ClusterManager),
		cfg.Server.EnableDevAPIs,
		cfg.Server.Auth.Enabled,
		middleware.Common()...,
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
		registerApplyRecommendationTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient, storageRepo)
		registerFetchMetricsTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient, storageRepo)
		registerNodeLoadMonitoringTask(ctx, cfg, clusterManager, clusterClients, clusterID, promClient)
		registerCleanupTask(ctx, cfg, clusterManager, clusterID, storageRepo)
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
			Name:                   clusterID + "_" + config.ApplyRecommendationKey,
			Enabled:                applyRecommendationTaskConfig.Enabled,
			Schedule:               applyRecommendationTaskConfig.Schedule,
			ClusterID:              clusterID,
			TargetClusterID:        cfg.Controller.TargetClusterID,
			TargetNamespace:        cfg.Controller.TargetNamespace,
			Auth:                   cfg.Server.Auth,
			RecommendationSettings: cfg.RecommendationSettings,
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
			Name:      clusterID + "_" + config.NodeLoadMonitoringKey,
			Enabled:   nodeLoadMonitoringTaskConfig.Enabled,
			Schedule:  nodeLoadMonitoringTaskConfig.Schedule,
			ClusterID: clusterID,
		},
	))
}

func registerCleanupTask(
	ctx context.Context,
	cfg *config.Config,
	clusterManager cluster.Manager,
	clusterID string,
	storageRepo *storage.Storage,
) {
	cleanupTaskConfig := cfg.GetTaskConfig(config.CleanupKey)

	clusterManager.AddTask(task.NewCleanupTask(
		ctx,
		storageRepo,
		&task.CleanupTaskConfig{
			Name:      clusterID + "_" + config.CleanupKey,
			Enabled:   cleanupTaskConfig.Enabled,
			Schedule:  cleanupTaskConfig.Schedule,
			ClusterID: clusterID,
		},
		cleanupTaskConfig,
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
			Name:      clusterID + "_" + config.DisruptionForceKey,
			Enabled:   disruptionForceTaskConfig.Enabled,
			Schedule:  disruptionForceTaskConfig.Schedule,
			ClusterID: clusterID,
		},
	))
}
