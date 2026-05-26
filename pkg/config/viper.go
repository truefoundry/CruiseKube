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
	"github.com/truefoundry/cruisekube/pkg/logging"
)

const defaultUsageTelemetryInterval = "30m"

// LoadWithViperInstance loads configuration using a provided Viper instance (for flag binding).
func LoadWithViperInstance(ctx context.Context, v *viper.Viper, configFilePath string) (*Config, error) {
	// Set defaults matching the new structure
	v.SetDefault("controllerMode", string(ClusterModeInCluster))
	v.SetDefault("executionMode", string(ExecutionModeBoth))
	v.SetDefault("dependencies.local.kubeconfigPath", "")
	v.SetDefault("dependencies.local.metricsProvider.type", "")
	v.SetDefault("dependencies.local.metricsProvider.url", "")
	v.SetDefault("dependencies.local.metricsProvider.bearerToken", "")
	v.SetDefault("dependencies.local.metricsProvider.insecureSkipTLSVerify", false)
	v.SetDefault("dependencies.inCluster.metricsProvider.type", "")
	v.SetDefault("dependencies.inCluster.metricsProvider.url", "")
	v.SetDefault("dependencies.inCluster.metricsProvider.bearerToken", "")
	v.SetDefault("dependencies.inCluster.metricsProvider.insecureSkipTLSVerify", false)
	v.SetDefault("controller.tasks.applyRecommendation.enabled", true)
	v.SetDefault("controller.tasks.applyRecommendation.schedule", "5m")
	v.SetDefault("controller.tasks.applyRecommendation.nodeStatsURL.host", "localhost:8080")
	v.SetDefault("controller.tasks.applyRecommendation.overridesURL.host", "localhost:8080")
	v.SetDefault("recommendationSettings.maxConcurrentQueries", 5)
	v.SetDefault("recommendationSettings.oomCooldownMinutes", 5)
	v.SetDefault("controller.tasks.cleanup.enabled", false)
	v.SetDefault("controller.tasks.cleanup.schedule", "24h")
	v.SetDefault("controller.tasks.cleanup.metadata.retentionDays", 7)
	v.SetDefault("server.auth.enabled", true)
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.enableDevAPIs", false)
	v.SetDefault("webhook.port", "8443")
	v.SetDefault("webhook.certsDir", "/certs")
	v.SetDefault("db.filePath", "cruisekube.db")
	v.SetDefault("telemetry.enabled", false)
	v.SetDefault("telemetry.traceRatio", 0.1)
	v.SetDefault("usageTelemetry.enabled", false)
	v.SetDefault("usageTelemetry.interval", defaultUsageTelemetryInterval)
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

func (c *Config) Validate(ctx context.Context) error {
	switch c.ExecutionMode {
	case ExecutionModeWebhook:
		return c.ValidateWebhookExecutionMode(ctx)
	case ExecutionModeController:
		return c.ValidateControllerExecutionMode(ctx)
	case ExecutionModeBoth:
		if err := c.ValidateWebhookExecutionMode(ctx); err != nil {
			return err
		}
		if err := c.ValidateControllerExecutionMode(ctx); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid execution-mode: %s (expected controller|webhook|both)", c.ExecutionMode)
	}
}

func (c *Config) ValidateWebhookExecutionMode(ctx context.Context) error {
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

func (c *Config) ValidateControllerExecutionMode(ctx context.Context) error {
	if _, err := c.ActiveMetricsProviderConfig(); err != nil {
		return err
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

	return c.validateUsageTelemetryForController(ctx)
}

// ActiveMetricsProviderConfig returns the metrics provider configuration for the
// active dependency block selected by controllerMode. An empty provider type is
// treated as prometheus.
func (c *Config) ActiveMetricsProviderConfig() (MetricsProviderConfig, error) {
	controllerMode := strings.TrimSpace(string(c.ControllerMode))
	switch controllerMode {
	case string(ClusterModeLocal):
		return resolveActiveMetricsProviderConfig(
			"dependencies.local",
			c.Dependencies.Local.MetricsProvider,
		)
	case string(ClusterModeInCluster):
		return resolveActiveMetricsProviderConfig(
			"dependencies.inCluster",
			c.Dependencies.InCluster.MetricsProvider,
		)
	default:
		return MetricsProviderConfig{}, fmt.Errorf("invalid controller-mode: %s (expected local|in-cluster)", controllerMode)
	}
}

func resolveActiveMetricsProviderConfig(configPath string, provider MetricsProviderConfig) (MetricsProviderConfig, error) {
	providerType := MetricsProviderType(strings.TrimSpace(string(provider.Type)))
	if providerType == "" {
		providerType = MetricsProviderTypePrometheus
	}
	provider.Type = providerType
	provider.URL = strings.TrimSpace(provider.URL)
	provider.BearerToken = strings.TrimSpace(provider.BearerToken)

	switch providerType {
	case MetricsProviderTypePrometheus:
		if provider.URL == "" {
			return MetricsProviderConfig{}, fmt.Errorf("%s.metricsProvider.url is required for prometheus metrics provider", configPath)
		}
		return provider, nil
	case MetricsProviderTypeKloudfuse:
		if provider.URL == "" {
			return MetricsProviderConfig{}, fmt.Errorf("%s.metricsProvider.url is required for kloudfuse metrics provider", configPath)
		}
		if provider.BearerToken == "" {
			return MetricsProviderConfig{}, fmt.Errorf("%s.metricsProvider.bearerToken is required for kloudfuse metrics provider", configPath)
		}
		return provider, nil
	default:
		return MetricsProviderConfig{}, fmt.Errorf("invalid metrics provider type %q at %s.metricsProvider.type (valid values: %s, %s)", providerType, configPath, MetricsProviderTypePrometheus, MetricsProviderTypeKloudfuse)
	}
}

func (c *Config) validateUsageTelemetryForController(ctx context.Context) error {
	if !c.UsageTelemetry.Enabled {
		return nil
	}
	interval := strings.TrimSpace(c.UsageTelemetry.Interval)
	if interval == "" {
		logging.Warnf(ctx, "usageTelemetry.enabled=true but interval is empty; defaulting to %s", defaultUsageTelemetryInterval)
		c.UsageTelemetry.Interval = defaultUsageTelemetryInterval
		interval = defaultUsageTelemetryInterval
	}
	d, err := time.ParseDuration(interval)
	if err != nil {
		logging.Warnf(ctx, "usageTelemetry.enabled=true but interval %q is invalid (%v); defaulting to %s", c.UsageTelemetry.Interval, err, defaultUsageTelemetryInterval)
		c.UsageTelemetry.Interval = defaultUsageTelemetryInterval
		d = 30 * time.Minute
	}
	if d <= 0 {
		logging.Warnf(ctx, "usageTelemetry.enabled=true but interval %q is non-positive; defaulting to %s", c.UsageTelemetry.Interval, defaultUsageTelemetryInterval)
		c.UsageTelemetry.Interval = defaultUsageTelemetryInterval
	}
	if strings.TrimSpace(c.UsageTelemetry.InstallID) == "" {
		logging.Warnf(ctx, "usageTelemetry.enabled=true but no install id found; disabling usage telemetry (set CRUISEKUBE_USAGETELEMETRY_INSTALLID to enable)")
		c.UsageTelemetry.Enabled = false
		return nil
	}
	if len(c.UsageTelemetry.ProviderConfig) == 0 {
		logging.Warnf(ctx, "usageTelemetry.enabled=true but providerConfig is empty; disabling usage telemetry (set usageTelemetry.providerConfig.host to enable)")
		c.UsageTelemetry.Enabled = false
		return nil
	}
	if c.EffectiveUsageTelemetryProviderAPIKey() == "" {
		logging.Warnf(ctx, "usageTelemetry.enabled=true but no provider API key found; disabling usage telemetry (set usageTelemetry.providerApiKey / CRUISEKUBE_USAGETELEMETRY_PROVIDERAPIKEY, providerConfig.api_key, or bake USAGETELEMETRY_PROVIDER_API_KEY at image build)")
		c.UsageTelemetry.Enabled = false
		return nil
	}
	return nil
}
