package config

import (
	"strings"
	"testing"
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
	delete(cfg.Controller.Tasks, "createStats")

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing task config")
	}
	if !strings.Contains(err.Error(), "missing required controller task configurations") {
		t.Fatalf("expected missing task config error, got %v", err)
	}
	if !strings.Contains(err.Error(), "createStats") {
		t.Fatalf("expected missing createStats task in error, got %v", err)
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

func TestGetTaskConfigSupportsLegacyInternalKeys(t *testing.T) {
	cfg := validControllerConfig()

	taskCfg := cfg.GetTaskConfig(CreateStatsKey)
	if taskCfg == nil {
		t.Fatal("expected createStats task config to resolve from internal key")
	}
	if taskCfg.Schedule != "15m" {
		t.Fatalf("expected createStats schedule to be 15m, got %q", taskCfg.Schedule)
	}
}

func validControllerConfig() *Config {
	return &Config{
		ControllerMode: ClusterModeInCluster,
		ExecutionMode:  ExecutionModeController,
		Dependencies: Dependencies{
			InCluster: InClusterDeps{
				PrometheusURL: "http://prometheus:9090",
			},
		},
		Controller: ControllerConfig{
			Tasks: map[string]*TaskConfig{
				"createStats": {
					Enabled:  true,
					Schedule: "15m",
				},
				"applyRecommendation": {
					Enabled:  true,
					Schedule: "5m",
				},
				"modifyEqualCPUResources": {
					Enabled:  false,
					Schedule: "10m",
				},
				"nodeLoadMonitoring": {
					Enabled:  false,
					Schedule: "60s",
				},
				"fetchMetrics": {
					Enabled:  true,
					Schedule: "1m",
				},
				"cleanupOOMEvent": {
					Enabled:  false,
					Schedule: "24h",
				},
				"disruptionForce": {
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
