package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateRejectsInvalidExecutionMode(t *testing.T) {
	cfg := validControllerConfig()
	cfg.ExecutionMode = ExecutionMode("invalid")

	err := cfg.Validate(context.Background())
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

	err := cfg.Validate(context.Background())
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

	err := cfg.Validate(context.Background())
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

	err := cfg.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for missing prometheus URL")
	}
	if !strings.Contains(err.Error(), "dependencies.inCluster.metricsProvider.url or dependencies.inCluster.prometheusURL is required") {
		t.Fatalf("expected missing dependencies.inCluster metrics provider URL error, got %v", err)
	}
}

func TestValidateRejectsMissingLocalPrometheusURL(t *testing.T) {
	cfg := validControllerConfig()
	cfg.ControllerMode = ClusterModeLocal
	cfg.Dependencies.Local.PrometheusURL = ""

	err := cfg.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for missing local prometheus URL")
	}
	if !strings.Contains(err.Error(), "dependencies.local.metricsProvider.url or dependencies.local.prometheusURL is required") {
		t.Fatalf("expected missing dependencies.local metrics provider URL error, got %v", err)
	}
}

func TestActiveMetricsProviderConfigLegacyPrometheusCompatibility(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.PrometheusURL = " http://legacy-prometheus:9090 "
	cfg.Dependencies.InCluster.InsecureSkipTLSVerify = true

	provider, err := cfg.ActiveMetricsProviderConfig()
	if err != nil {
		t.Fatalf("expected active provider config, got %v", err)
	}
	if provider.Type != MetricsProviderTypePrometheus {
		t.Fatalf("expected prometheus provider type, got %q", provider.Type)
	}
	if provider.URL != "http://legacy-prometheus:9090" {
		t.Fatalf("expected legacy prometheus URL, got %q", provider.URL)
	}
	if provider.BearerToken != "" {
		t.Fatalf("expected empty prometheus bearer token, got %q", provider.BearerToken)
	}
	if !provider.InsecureSkipTLSVerify {
		t.Fatal("expected legacy insecureSkipTLSVerify to be preserved")
	}
}

func TestActiveMetricsProviderConfigPrometheusUsesMetricsProviderURL(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.PrometheusURL = "http://legacy-prometheus:9090"
	cfg.Dependencies.InCluster.MetricsProvider = MetricsProviderConfig{
		Type:        MetricsProviderTypePrometheus,
		URL:         " http://configured-prometheus:9090 ",
		BearerToken: " prometheus-token ",
	}

	provider, err := cfg.ActiveMetricsProviderConfig()
	if err != nil {
		t.Fatalf("expected active provider config, got %v", err)
	}
	if provider.URL != "http://configured-prometheus:9090" {
		t.Fatalf("expected metricsProvider URL to win, got %q", provider.URL)
	}
	if provider.BearerToken != "prometheus-token" {
		t.Fatalf("expected prometheus bearer token to be preserved when configured, got %q", provider.BearerToken)
	}
}

func TestActiveMetricsProviderConfigPrometheusLegacyURLPreservesStructuredInsecureSkipTLSVerify(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.PrometheusURL = " https://legacy-prometheus.example "
	cfg.Dependencies.InCluster.InsecureSkipTLSVerify = false
	cfg.Dependencies.InCluster.MetricsProvider = MetricsProviderConfig{
		Type:                  MetricsProviderTypePrometheus,
		InsecureSkipTLSVerify: true,
	}

	provider, err := cfg.ActiveMetricsProviderConfig()
	if err != nil {
		t.Fatalf("expected active provider config, got %v", err)
	}
	if provider.URL != "https://legacy-prometheus.example" {
		t.Fatalf("expected legacy prometheus URL, got %q", provider.URL)
	}
	if !provider.InsecureSkipTLSVerify {
		t.Fatal("expected structured insecureSkipTLSVerify to be preserved when URL falls back to legacy prometheusURL")
	}
}

func TestValidateRejectsKloudfuseMissingURL(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.MetricsProvider = MetricsProviderConfig{
		Type:        MetricsProviderTypeKloudfuse,
		BearerToken: "token",
	}

	err := cfg.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for missing kloudfuse URL")
	}
	if !strings.Contains(err.Error(), "dependencies.inCluster.metricsProvider.url is required for kloudfuse") {
		t.Fatalf("expected missing kloudfuse URL error, got %v", err)
	}
}

func TestValidateRejectsKloudfuseMissingBearerToken(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.MetricsProvider = MetricsProviderConfig{
		Type: MetricsProviderTypeKloudfuse,
		URL:  "https://kloudfuse.example.com",
	}

	err := cfg.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for missing kloudfuse bearer token")
	}
	if !strings.Contains(err.Error(), "dependencies.inCluster.metricsProvider.bearerToken is required for kloudfuse") {
		t.Fatalf("expected missing kloudfuse bearer token error, got %v", err)
	}
}

func TestValidateAllowsValidKloudfuse(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.MetricsProvider = MetricsProviderConfig{
		Type:                  MetricsProviderTypeKloudfuse,
		URL:                   "https://kloudfuse.example.com",
		BearerToken:           "token",
		InsecureSkipTLSVerify: true,
	}

	if err := cfg.Validate(context.Background()); err != nil {
		t.Fatalf("expected valid kloudfuse config, got %v", err)
	}

	provider, err := cfg.ActiveMetricsProviderConfig()
	if err != nil {
		t.Fatalf("expected active provider config, got %v", err)
	}
	if provider.Type != MetricsProviderTypeKloudfuse || provider.URL != "https://kloudfuse.example.com" || provider.BearerToken != "token" || !provider.InsecureSkipTLSVerify {
		t.Fatalf("unexpected kloudfuse provider config: %+v", provider)
	}
}

func TestValidateRejectsUnknownMetricsProviderType(t *testing.T) {
	cfg := validControllerConfig()
	cfg.Dependencies.InCluster.MetricsProvider.Type = MetricsProviderType("other")

	err := cfg.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for unknown provider type")
	}
	if !strings.Contains(err.Error(), "invalid metrics provider type") || !strings.Contains(err.Error(), "prometheus") || !strings.Contains(err.Error(), "kloudfuse") {
		t.Fatalf("expected unknown provider type error with valid values, got %v", err)
	}
}

func TestLoadHydratesInClusterMetricsProviderEnv(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_TYPE", "kloudfuse")
	t.Setenv("CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_URL", "https://kloudfuse.example.com")
	t.Setenv("CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_BEARERTOKEN", "token")
	t.Setenv("CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_INSECURESKIPTLSVERIFY", "true")

	cfg, err := LoadWithViperInstance(context.Background(), viper.New(), configPath)
	if err != nil {
		t.Fatal(err)
	}
	provider := cfg.Dependencies.InCluster.MetricsProvider
	if provider.Type != MetricsProviderTypeKloudfuse {
		t.Fatalf("expected type from env, got %q", provider.Type)
	}
	if provider.URL != "https://kloudfuse.example.com" {
		t.Fatalf("expected url from env, got %q", provider.URL)
	}
	if provider.BearerToken != "token" {
		t.Fatalf("expected bearer token from env, got %q", provider.BearerToken)
	}
	if !provider.InsecureSkipTLSVerify {
		t.Fatal("expected insecureSkipTLSVerify from env")
	}
}

func TestValidateAllowsUsageTelemetryWithoutProviderAPIKey(t *testing.T) {
	t.Parallel()
	cfg := validControllerConfig()
	cfg.UsageTelemetry = UsageTelemetryConfig{
		Enabled:        true,
		Interval:       "30m",
		InstallID:      "install",
		ProviderConfig: map[string]interface{}{"host": "https://us.i.posthog.com"},
	}
	err := cfg.Validate(context.Background())
	if err != nil {
		t.Fatalf("expected validation to pass even when provider API key is missing, got %v", err)
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
	if err := cfg.Validate(context.Background()); err != nil {
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
				ProviderAPIKey: "x",
				ProviderConfig: map[string]interface{}{"host": "https://us.i.posthog.com"},
			}
			err := cfg.Validate(context.Background())
			if err != nil {
				t.Fatalf("expected validation to pass (interval should default), got %v", err)
			}
			if cfg.UsageTelemetry.Interval != "30m" {
				t.Fatalf("expected interval to default to 30m, got %q", cfg.UsageTelemetry.Interval)
			}
		})
	}
}

func TestValidateRejectsMissingWebhookFields(t *testing.T) {
	cfg := validWebhookConfig()
	cfg.Webhook.Port = ""
	cfg.Webhook.CertsDir = ""

	err := cfg.Validate(context.Background())
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
