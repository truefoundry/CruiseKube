package config

const (
	ApplyRecommendationKey = "applyrecommendation"
	FetchMetricsKey        = "fetchmetrics"
	CreateStatsKey         = "createstats"
	NodeLoadMonitoringKey  = "nodeloadmonitoring"
	CleanupKey             = "cleanup"
	DisruptionForceKey     = "disruptionforce"
)

type Config struct {
	ControllerMode         ControllerMode         `yaml:"controllerMode" mapstructure:"controllerMode"`
	Dependencies           Dependencies           `yaml:"dependencies" mapstructure:"dependencies"`
	ExecutionMode          ExecutionMode          `yaml:"executionMode" mapstructure:"executionMode"`
	Controller             ControllerConfig       `yaml:"controller" mapstructure:"controller"`
	Server                 ServerConfig           `yaml:"server" mapstructure:"server"`
	Webhook                WebhookConfig          `yaml:"webhook" mapstructure:"webhook"`
	DB                     DatabaseConfig         `yaml:"db" mapstructure:"db"`
	RecommendationSettings RecommendationSettings `yaml:"recommendationSettings" mapstructure:"recommendationSettings"`
	Telemetry              TelemetryConfig        `yaml:"telemetry" mapstructure:"telemetry"`
	UsageTelemetry         UsageTelemetryConfig   `yaml:"usageTelemetry" mapstructure:"usageTelemetry"`
	Metrics                MetricsConfig          `yaml:"metrics" mapstructure:"metrics"`
	Sentry                 SentryConfig           `yaml:"sentry" mapstructure:"sentry"`
	Custom                 map[string]interface{} `yaml:",inline" mapstructure:",remain"`
}

func (c *Config) GetTaskConfig(taskName string) *TaskConfig {
	if c == nil || c.Controller.Tasks == nil {
		return nil
	}

	return c.Controller.Tasks[taskName]
}

func RequiredTaskKeys() []string {
	return []string{
		CreateStatsKey,
		ApplyRecommendationKey,
		NodeLoadMonitoringKey,
		FetchMetricsKey,
		CleanupKey,
		DisruptionForceKey,
	}
}

type Dependencies struct {
	Local     LocalDeps     `yaml:"local" mapstructure:"local"`
	InCluster InClusterDeps `yaml:"inCluster" mapstructure:"inCluster"`
}

type LocalDeps struct {
	KubeconfigPath        string `yaml:"kubeconfigPath" mapstructure:"kubeconfigPath"`
	PrometheusURL         string `yaml:"prometheusURL" mapstructure:"prometheusURL"`
	InsecureSkipTLSVerify bool   `yaml:"insecureSkipTLSVerify" mapstructure:"insecureSkipTLSVerify"`
}

type InClusterDeps struct {
	PrometheusURL         string `yaml:"prometheusURL" mapstructure:"prometheusURL"`
	InsecureSkipTLSVerify bool   `yaml:"insecureSkipTLSVerify" mapstructure:"insecureSkipTLSVerify"`
}

type ControllerConfig struct {
	TargetNamespace string                 `yaml:"targetNamespace,omitempty" mapstructure:"targetNamespace"`
	TargetClusterID string                 `yaml:"targetClusterID,omitempty" mapstructure:"targetClusterID"`
	Tasks           map[string]*TaskConfig `yaml:"tasks" mapstructure:"tasks"`
}

type URLConfig struct {
	Host            string `yaml:"host" mapstructure:"host"`
	TfyClusterToken string `yaml:"tfyClusterToken" mapstructure:"tfyClusterToken"`
}

type ServerConfig struct {
	Port          string     `yaml:"port" mapstructure:"port"`
	Auth          AuthConfig `yaml:"auth" mapstructure:"auth"`
	EnableDevAPIs bool       `yaml:"enableDevAPIs" mapstructure:"enableDevAPIs"`
}

type AuthConfig struct {
	Username string `yaml:"username" mapstructure:"username"`
	Password string `yaml:"password" mapstructure:"password"`
}

type WebhookConfig struct {
	Port     string    `yaml:"port" mapstructure:"port"`
	CertsDir string    `yaml:"certsDir" mapstructure:"certsDir"`
	StatsURL URLConfig `yaml:"statsURL" mapstructure:"statsURL"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type" json:"type"`         // "sqlite" or "postgres"
	Host     string `yaml:"host" json:"host"`         // For postgres
	Port     int    `yaml:"port" json:"port"`         // For postgres
	Database string `yaml:"database" json:"database"` // Database name or file path
	Username string `yaml:"username" json:"username"` // For postgres
	Password string `yaml:"password" json:"password"` // For postgres
	SSLMode  string `yaml:"sslmode" json:"sslmode"`   // For postgres
}

type RecommendationSettings struct {
	NewWorkloadThresholdHours int  `yaml:"newWorkloadThresholdHours" mapstructure:"newWorkloadThresholdHours"`
	DisableMemoryApplication  bool `yaml:"disableMemoryApplication" mapstructure:"disableMemoryApplication"`
	MaxConcurrentQueries      int  `yaml:"maxConcurrentQueries" mapstructure:"maxConcurrentQueries"`
	OOMCooldownMinutes        int  `yaml:"oomCooldownMinutes" mapstructure:"oomCooldownMinutes"`
	OptimizeGuaranteedPods    bool `yaml:"optimizeGuaranteedPods" mapstructure:"optimizeGuaranteedPods"`
}

type TelemetryConfig struct {
	Enabled              bool    `yaml:"enabled" mapstructure:"enabled"`
	ExporterOTLPEndpoint string  `yaml:"exporterOTLPEndpoint" mapstructure:"exporterOTLPEndpoint"`
	ExporterOTLPHeaders  string  `yaml:"exporterOTLPHeaders" mapstructure:"exporterOTLPHeaders"`
	ServiceName          string  `yaml:"serviceName" mapstructure:"serviceName"`
	TraceRatio           float64 `yaml:"traceRatio" mapstructure:"traceRatio"`
}

// UsageTelemetryConfig controls optional product usage heartbeats (distinct from OTLP telemetry).
type UsageTelemetryConfig struct {
	Enabled          bool                   `yaml:"enabled" mapstructure:"enabled"`
	Interval         string                 `yaml:"interval" mapstructure:"interval"`
	InstallID        string                 `yaml:"installID" mapstructure:"installID"`
	ProviderAPIKey   string                 `yaml:"providerApiKey,omitempty" mapstructure:"providerApiKey"`
	ProviderConfig   map[string]interface{} `yaml:"providerConfig" mapstructure:"providerConfig"`
	HelmChartVersion string                 `yaml:"helmChartVersion" mapstructure:"helmChartVersion"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Port    string `yaml:"port" mapstructure:"port"`
}

type SentryConfig struct {
	Enabled     bool   `yaml:"enabled" mapstructure:"enabled"`
	DSN         string `yaml:"dsn" mapstructure:"dsn"`
	Environment string `yaml:"environment" mapstructure:"environment"`
}

type ControllerMode string

const (
	ClusterModeLocal     ControllerMode = "local"
	ClusterModeInCluster ControllerMode = "in-cluster"
)

type ExecutionMode string

const (
	ExecutionModeController ExecutionMode = "controller"
	ExecutionModeWebhook    ExecutionMode = "webhook"
	ExecutionModeBoth       ExecutionMode = "both"
)
