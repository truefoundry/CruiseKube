package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateRejectsInvalidExecutionMode(t *testing.T) {
	cfg := validControllerConfig()
	cfg.ExecutionMode = ExecutionMode("invalid")

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid execution mode")
	}
	if !strings.Contains(err.Error(), "invalid execution-mode") {
		t.Fatalf("expected invalid execution-mode error, got %v", err)
	}
}

func TestValidateRejectsInvalidControllerMode(t *testing.T) {
	cfg := validControllerConfig()
	cfg.ControllerMode = ControllerMode("invalid")

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid controller mode")
	}
	if !strings.Contains(err.Error(), "invalid controller-mode") {
		t.Fatalf("expected invalid controller-mode error, got %v", err)
	}
}

func TestValidateRejectsMissingTaskConfigs(t *testing.T) {
	cfg := validControllerConfig()
	delete(cfg.Controller.Tasks, CreateStatsKey)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing task config")
	}
	if !strings.Contains(err.Error(), "missing required controller task configurations") {
		t.Fatalf("expected missing task config error, got %v", err)
	}
	if !strings.Contains(err.Error(), CreateStatsKey) {
		t.Fatalf("expected missing %s task in error, got %v", CreateStatsKey, err)
	}
}

func TestValidateRejectsMissingInClusterPrometheusURL(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.PrometheusURL = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing prometheus URL")
	}
	if !strings.Contains(err.Error(), "dependencies.inCluster.prometheusURL is required") {
		t.Fatalf("expected missing dependencies.inCluster.prometheusURL error, got %v", err)
	}
}

func TestValidateRejectsMissingLocalPrometheusURL(t *testing.T) {
	cfg := validControllerConfig()
	cfg.ControllerMode = ClusterModeLocal
	cfg.Dependencies.Local.PrometheusURL = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing local prometheus URL")
	}
	if !strings.Contains(err.Error(), "dependencies.local.prometheusURL is required") {
		t.Fatalf("expected missing dependencies.local.prometheusURL error, got %v", err)
	}
}

func TestValidateRequiresUsageTelemetryProviderAPIKey(t *testing.T) {
	t.Parallel()
	cfg := validControllerConfig()
	cfg.UsageTelemetry = UsageTelemetryConfig{
		Enabled:        true,
		Interval:       "30m",
		InstallID:      "install",
		ProviderConfig: map[string]interface{}{"host": "https://us.i.posthog.com"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error when api_key is missing")
	}
	if !strings.Contains(err.Error(), "provider API key") {
		t.Fatalf("expected provider API key error, got %v", err)
	}
}

func TestValidateAllowsUsageTelemetryAfterHydrateWithProviderApiKeyField(t *testing.T) {
	t.Parallel()
	cfg := validControllerConfig()
	cfg.UsageTelemetry = UsageTelemetryConfig{
		Enabled:        true,
		Interval:       "30m",
		InstallID:      "install",
		ProviderAPIKey: "phc_test",
		ProviderConfig: map[string]interface{}{"host": "https://us.i.posthog.com"},
	}
	v := viper.New()
	if err := hydrateUsageTelemetryProviderConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUsageTelemetryNonPositiveInterval(t *testing.T) {
	t.Parallel()
	for _, interval := range []string{"0s", "0", "-5m", "-1ns"} {
		t.Run(interval, func(t *testing.T) {
			t.Parallel()
			cfg := validControllerConfig()
			cfg.UsageTelemetry = UsageTelemetryConfig{
				Enabled:        true,
				Interval:       interval,
				InstallID:      "abc",
				ProviderConfig: map[string]interface{}{"api_key": "x"},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "usageTelemetry.interval must be positive") {
				t.Fatalf("expected positive interval error, got %v", err)
			}
		})
	}
}

func TestValidateRejectsMissingWebhookFields(t *testing.T) {
	cfg := validWebhookConfig()
	cfg.Webhook.Port = ""
	cfg.Webhook.CertsDir = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing webhook fields")
	}
	if !strings.Contains(err.Error(), "missing required webhook configuration values") {
		t.Fatalf("expected missing webhook fields error, got %v", err)
	}
	if !strings.Contains(err.Error(), "webhook.port") || !strings.Contains(err.Error(), "webhook.certsDir") {
		t.Fatalf("expected missing webhook field names in error, got %v", err)
	}
}

func validControllerConfig() *Config {
	return &Config{
		ControllerMode: ClusterModeInCluster,
		ExecutionMode:  ExecutionModeController,
		Dependencies: Dependencies{
			Local: LocalDeps{
				PrometheusURL:         "http://localhost:9090",
				InsecureSkipTLSVerify: false,
			},
			InCluster: InClusterDeps{
				PrometheusURL:         "http://prometheus:9090",
				InsecureSkipTLSVerify: false,
			},
		},
		Controller: ControllerConfig{
			Tasks: map[string]*TaskConfig{
				CreateStatsKey: {
					Enabled:  true,
					Schedule: "15m",
				},
				ApplyRecommendationKey: {
					Enabled:  true,
					Schedule: "5m",
				},
				NodeLoadMonitoringKey: {
					Enabled:  false,
					Schedule: "60s",
				},
				FetchMetricsKey: {
					Enabled:  true,
					Schedule: "1m",
				},
				CleanupKey: {
					Enabled:  false,
					Schedule: "24h",
				},
				DisruptionForceKey: {
					Enabled:  true,
					Schedule: "5m",
				},
			},
		},
		Webhook: WebhookConfig{
			Port:     "8443",
			CertsDir: "/certs",
			StatsURL: URLConfig{Host: "http://stats"},
		},
	}
}

func validWebhookConfig() *Config {
	cfg := validControllerConfig()
	cfg.ExecutionMode = ExecutionModeWebhook
	return cfg
}
