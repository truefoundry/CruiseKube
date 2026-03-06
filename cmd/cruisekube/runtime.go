package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/server"
	"github.com/truefoundry/cruisekube/pkg/telemetry"

	"github.com/spf13/cobra"
)

func runCruiseKube(cmd *cobra.Command, args []string) (runErr error) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	runtime := newRuntimeManager(ctx)
	defer func() {
		if shutdownErr := runtime.Shutdown(); shutdownErr != nil {
			if runErr == nil {
				runErr = fmt.Errorf("shutdown failed: %w", shutdownErr)
				return
			}

			logging.Errorf(context.Background(), "Failed to shutdown runtime: %v", shutdownErr)
		}
	}()

	cfg, err := loadRuntimeConfig(ctx)
	if err != nil {
		return err
	}

	if err := setupTelemetry(runtime, cfg); err != nil {
		return err
	}
	startMetricsServer(runtime, cfg)

	if shouldStartWebhook(cfg.ExecutionMode) {
		startWebhookRuntime(runtime, cfg)
	}

	if shouldStartController(cfg.ExecutionMode) {
		if err := startControllerRuntime(runtime, cfg); err != nil {
			return err
		}
	}

	return runtime.Wait()
}

func loadRuntimeConfig(ctx context.Context) (*config.Config, error) {
	cfg, err := config.LoadWithViperInstance(ctx, v, configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	logging.Infof(ctx, "Configuration loaded: controllerMode=%s executionMode=%s", cfg.ControllerMode, cfg.ExecutionMode)
	return cfg, nil
}

func setupTelemetry(runtime *runtimeManager, cfg *config.Config) error {
	if !cfg.Telemetry.Enabled {
		return nil
	}

	shutdown, err := telemetry.Init(runtime.ctx, cfg.Telemetry)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}

	runtime.AddCleanup(shutdown)
	return nil
}

func startMetricsServer(runtime *runtimeManager, cfg *config.Config) {
	if !cfg.Metrics.Enabled {
		return
	}

	metricsServer := &http.Server{
		Addr:              ":" + cfg.Metrics.Port,
		Handler:           server.SetupMetricsServerEngine(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	startHTTPServer(runtime, "metrics server", "Starting metrics server on :"+cfg.Metrics.Port, metricsServer, func(server *http.Server) error {
		return server.ListenAndServe()
	})
}

func shouldStartWebhook(mode config.ExecutionMode) bool {
	return mode == config.ExecutionModeWebhook || mode == config.ExecutionModeBoth
}

func shouldStartController(mode config.ExecutionMode) bool {
	return mode == config.ExecutionModeController || mode == config.ExecutionModeBoth
}

func startHTTPServer(runtime *runtimeManager, name string, startupMessage string, httpServer *http.Server, serve func(*http.Server) error) {
	runtime.AddCleanup(func(ctx context.Context) error {
		return httpServer.Shutdown(ctx)
	})

	runtime.Go(name, func(ctx context.Context) error {
		logging.Infof(ctx, "%s", startupMessage)

		err := serve(httpServer)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	})
}
