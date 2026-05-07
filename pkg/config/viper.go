package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LoadWithViperInstance loads configuration using a provided Viper instance (for flag binding).
func LoadWithViperInstance(ctx context.Context, v *viper.Viper, configFilePath string) (*Config, error) {
	// Set defaults matching the new structure
	v.SetDefault("controllerMode", string(ClusterModeInCluster))
	v.SetDefault("executionMode", string(ExecutionModeBoth))
	v.SetDefault("dependencies.local.kubeconfigPath", "")
	v.SetDefault("dependencies.local.prometheusURL", "")
	v.SetDefault("dependencies.local.insecureSkipTLSVerify", false)
	v.SetDefault("dependencies.inCluster.prometheusURL", "")
	v.SetDefault("dependencies.inCluster.insecureSkipTLSVerify", false)
	v.SetDefault("controller.tasks.applyRecommendation.enabled", true)
	v.SetDefault("controller.tasks.applyRecommendation.schedule", "5m")
	v.SetDefault("controller.tasks.applyRecommendation.nodeStatsURL.host", "localhost:8080")
	v.SetDefault("controller.tasks.applyRecommendation.overridesURL.host", "localhost:8080")
	v.SetDefault("recommendationSettings.maxConcurrentQueries", 5)
	v.SetDefault("recommendationSettings.oomCooldownMinutes", 5)
	v.SetDefault("controller.tasks.cleanup.enabled", false)
	v.SetDefault("controller.tasks.cleanup.schedule", "24h")
	v.SetDefault("controller.tasks.cleanup.metadata.retentionDays", 7)
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.enableDevAPIs", false)
	v.SetDefault("webhook.port", "8443")
	v.SetDefault("webhook.certsDir", "/certs")
	v.SetDefault("db.filePath", "cruisekube.db")
	v.SetDefault("telemetry.enabled", false)
	v.SetDefault("telemetry.traceRatio", 0.1)
	v.SetDefault("usageTelemetry.enabled", false)
	v.SetDefault("usageTelemetry.interval", "30m")
	v.SetDefault("usageTelemetry.installID", "")
	// providerConfig must come from config file, Helm (CRUISEKUBE_USAGETELEMETRY_PROVIDERCONFIG JSON), or other env; no defaults with secrets.
	v.SetDefault("usageTelemetry.helmChartVersion", "")

	v.SetConfigType("yaml")
	v.SetConfigFile(configFilePath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFilePath, err)
	}

	v.SetEnvPrefix("cruisekube")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := hydrateUsageTelemetryProviderConfig(v, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// hydrateUsageTelemetryProviderConfig fills providerConfig when Unmarshal left it empty:
// 1) CRUISEKUBE_USAGETELEMETRY_PROVIDERCONFIG is a JSON object string (Helm toJson), or
// 2) Viper has nested keys under usageTelemetry.providerConfig (e.g. from env).
func hydrateUsageTelemetryProviderConfig(v *viper.Viper, cfg *Config) error {
	if len(cfg.UsageTelemetry.ProviderConfig) > 0 {
		return nil
	}
	raw := strings.TrimSpace(v.GetString("usageTelemetry.providerConfig"))
	if raw != "" && strings.HasPrefix(raw, "{") {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return fmt.Errorf("usageTelemetry.providerConfig: invalid JSON: %w", err)
		}
		cfg.UsageTelemetry.ProviderConfig = parsed
		return nil
	}
	if sm := v.GetStringMap("usageTelemetry.providerConfig"); len(sm) > 0 {
		cfg.UsageTelemetry.ProviderConfig = sm
	}
	return nil
}

func (c *Config) Validate() error {
	switch c.ExecutionMode {
	case ExecutionModeWebhook:
		return c.ValidateWebhookExecutionMode()
	case ExecutionModeController:
		return c.ValidateControllerExecutionMode()
	case ExecutionModeBoth:
		if err := c.ValidateWebhookExecutionMode(); err != nil {
			return err
		}
		if err := c.ValidateControllerExecutionMode(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid execution-mode: %s (expected controller|webhook|both)", c.ExecutionMode)
	}
}

func (c *Config) ValidateWebhookExecutionMode() error {
	var missing []string
	if c.Webhook.Port == "" {
		missing = append(missing, "webhook.port")
	}
	if c.Webhook.StatsURL.Host == "" {
		missing = append(missing, "webhook.statsURL.host")
	}
	if c.Webhook.CertsDir == "" {
		missing = append(missing, "webhook.certsDir")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required webhook configuration values: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c *Config) ValidateControllerExecutionMode() error {
	controllerMode := strings.TrimSpace(string(c.ControllerMode))
	switch controllerMode {
	case string(ClusterModeLocal):
		if strings.TrimSpace(c.Dependencies.Local.PrometheusURL) == "" {
			return fmt.Errorf("dependencies.local.prometheusURL is required in local mode")
		}
	case string(ClusterModeInCluster):
		if strings.TrimSpace(c.Dependencies.InCluster.PrometheusURL) == "" {
			return fmt.Errorf("dependencies.inCluster.prometheusURL is required in inCluster mode")
		}
	default:
		return fmt.Errorf("invalid controller-mode: %s (expected local|in-cluster)", controllerMode)
	}

	var missingTaskConfigs []string
	for _, taskKey := range RequiredTaskKeys() {
		if c.GetTaskConfig(taskKey) == nil {
			missingTaskConfigs = append(missingTaskConfigs, taskKey)
		}
	}
	if len(missingTaskConfigs) > 0 {
		return fmt.Errorf("missing required controller task configurations: %s", strings.Join(missingTaskConfigs, ", "))
	}

	return c.validateUsageTelemetryForController()
}

func (c *Config) validateUsageTelemetryForController() error {
	if !c.UsageTelemetry.Enabled {
		return nil
	}
	interval := strings.TrimSpace(c.UsageTelemetry.Interval)
	if interval == "" {
		return fmt.Errorf("usageTelemetry.interval is required when usageTelemetry.enabled is true")
	}
	d, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("usageTelemetry.interval: %w", err)
	}
	if d <= 0 {
		return fmt.Errorf("usageTelemetry.interval must be positive, got %q", c.UsageTelemetry.Interval)
	}
	if strings.TrimSpace(c.UsageTelemetry.InstallID) == "" {
		return fmt.Errorf("usageTelemetry.installID is required when usageTelemetry.enabled is true (set CRUISEKUBE_USAGETELEMETRY_INSTALLID)")
	}
	if len(c.UsageTelemetry.ProviderConfig) == 0 {
		return fmt.Errorf("usageTelemetry.providerConfig must be non-empty when usageTelemetry.enabled is true")
	}
	return nil
}
