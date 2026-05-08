package config

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
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

// hydrateUsageTelemetryProviderConfig normalizes usageTelemetry.providerConfig from YAML,
// CRUISEKUBE_USAGETELEMETRY_PROVIDERCONFIG JSON, or viper string map (e.g. host).
func hydrateUsageTelemetryProviderConfig(v *viper.Viper, cfg *Config) error {
	var m map[string]interface{}
	if len(cfg.UsageTelemetry.ProviderConfig) > 0 {
		m = maps.Clone(cfg.UsageTelemetry.ProviderConfig)
	} else {
		raw := strings.TrimSpace(v.GetString("usageTelemetry.providerConfig"))
		if raw != "" && strings.HasPrefix(raw, "{") {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				return fmt.Errorf("usageTelemetry.providerConfig: invalid JSON: %w", err)
			}
			m = parsed
		} else if sm := v.GetStringMap("usageTelemetry.providerConfig"); len(sm) > 0 {
			m = maps.Clone(sm)
		}
	}
	if m == nil {
		m = map[string]interface{}{}
	}

	cfg.UsageTelemetry.ProviderConfig = m
	return nil
}

// EffectiveUsageTelemetryProviderAPIKey returns the API key for usage telemetry: usageTelemetry.providerApiKey
// (or CRUISEKUBE_USAGETELEMETRY_PROVIDERAPIKEY), else the link-time image default, else providerConfig.api_key from YAML/JSON if present.
func (c *Config) EffectiveUsageTelemetryProviderAPIKey() string {
	if k := strings.TrimSpace(c.UsageTelemetry.ProviderAPIKey); k != "" {
		return k
	}
	if k := strings.TrimSpace(buildmetadata.DefaultUsageTelemetryProviderAPIKey); k != "" {
		return k
	}
	return usageTelemetryStringFromMap(c.UsageTelemetry.ProviderConfig, "api_key")
}

func usageTelemetryStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	default:
		return strings.TrimSpace(fmt.Sprint(s))
	}
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
	if c.EffectiveUsageTelemetryProviderAPIKey() == "" {
		return fmt.Errorf("usageTelemetry provider API key is required when usageTelemetry.enabled is true (set usageTelemetry.providerApiKey / CRUISEKUBE_USAGETELEMETRY_PROVIDERAPIKEY, providerConfig.api_key in YAML, or bake USAGETELEMETRY_PROVIDER_API_KEY at image build)")
	}
	return nil
}
