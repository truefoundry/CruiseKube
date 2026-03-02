package main

import (
	"context"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/server"
	"github.com/truefoundry/cruisekube/pkg/telemetry"

	"github.com/spf13/cobra"
)

func runCruiseKube(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := loadRuntimeConfig(ctx)

	if shutdownTelemetry := setupTelemetry(ctx, cfg); shutdownTelemetry != nil {
		defer shutdownTelemetry()
	}
	startMetricsServer(ctx, cfg)

	if shouldStartWebhook(cfg.ExecutionMode) {
		startWebhookRuntime(ctx, cfg)
	}

	if shouldStartController(cfg.ExecutionMode) {
		startControllerRuntime(ctx, cfg)
	}

	if shouldBlockForever(cfg.ExecutionMode) {
		blockForever()
	}
}

func loadRuntimeConfig(ctx context.Context) *config.Config {
	cfg, err := config.LoadWithViperInstance(ctx, v, configFilePath)
	if err != nil {
		logging.Fatalf(ctx, "Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		logging.Fatalf(ctx, "Invalid configuration: %v", err)
	}

	logging.Infof(ctx, "Configuration loaded: controllerMode=%s executionMode=%s", cfg.ControllerMode, cfg.ExecutionMode)
	return cfg
}

func setupTelemetry(ctx context.Context, cfg *config.Config) func() {
	if !cfg.Telemetry.Enabled {
		return nil
	}

	shutdown, err := telemetry.Init(ctx, cfg.Telemetry)
	if err != nil {
		logging.Fatalf(ctx, "Failed to initialize telemetry: %v", err)
	}

	return func() {
		if err := shutdown(ctx); err != nil {
			logging.Errorf(ctx, "Failed to shutdown telemetry: %v", err)
		}
	}
}

func startMetricsServer(ctx context.Context, cfg *config.Config) {
	if !cfg.Metrics.Enabled {
		return
	}

	metricsEngine := server.SetupMetricsServerEngine()
	metricsPort := cfg.Metrics.Port

	go func() {
		logging.Infof(ctx, "Starting metrics server on :%s", metricsPort)
		if err := metricsEngine.Run(":" + metricsPort); err != nil {
			logging.Fatalf(ctx, "Metrics server failed: %v", err)
		}
	}()
}

func shouldStartWebhook(mode config.ExecutionMode) bool {
	return mode == config.ExecutionModeWebhook || mode == config.ExecutionModeBoth
}

func shouldStartController(mode config.ExecutionMode) bool {
	return mode == config.ExecutionModeController || mode == config.ExecutionModeBoth
}

func shouldBlockForever(mode config.ExecutionMode) bool {
	return mode == config.ExecutionModeWebhook
}
