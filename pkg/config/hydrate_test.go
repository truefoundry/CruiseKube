package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
)

func TestHydrateUsageTelemetryProviderConfig_doesNotInjectProviderApiKey(t *testing.T) {
	t.Parallel()
	v := viper.New()
	cfg := &Config{
		UsageTelemetry: UsageTelemetryConfig{
			ProviderConfig: map[string]interface{}{"host": "https://x.example"},
			ProviderAPIKey: "key-from-struct",
		},
	}
	if err := hydrateUsageTelemetryProviderConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "api_key"); got != "" {
		t.Fatalf("expected no api_key in providerConfig, got %q", got)
	}
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "host"); got != "https://x.example" {
		t.Fatalf("host=%q", got)
	}
	if got := cfg.EffectiveUsageTelemetryProviderAPIKey(); got != "key-from-struct" {
		t.Fatalf("effective key=%q", got)
	}
}

func TestEffectiveUsageTelemetryProviderAPIKey_bakedFallback(t *testing.T) {
	t.Parallel()
	prev := buildmetadata.DefaultUsageTelemetryProviderAPIKey
	buildmetadata.DefaultUsageTelemetryProviderAPIKey = "baked-key"
	t.Cleanup(func() { buildmetadata.DefaultUsageTelemetryProviderAPIKey = prev })

	v := viper.New()
	cfg := &Config{
		UsageTelemetry: UsageTelemetryConfig{
			ProviderConfig: map[string]interface{}{"host": "https://x.example"},
		},
	}
	if err := hydrateUsageTelemetryProviderConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "api_key"); got != "" {
		t.Fatalf("expected no api_key in providerConfig after hydrate")
	}
	if got := cfg.EffectiveUsageTelemetryProviderAPIKey(); got != "baked-key" {
		t.Fatalf("effective key=%q", got)
	}
}

func TestEffectiveUsageTelemetryProviderAPIKey_explicitWinsOverBaked(t *testing.T) {
	t.Parallel()
	prev := buildmetadata.DefaultUsageTelemetryProviderAPIKey
	buildmetadata.DefaultUsageTelemetryProviderAPIKey = "baked"
	t.Cleanup(func() { buildmetadata.DefaultUsageTelemetryProviderAPIKey = prev })

	v := viper.New()
	cfg := &Config{
		UsageTelemetry: UsageTelemetryConfig{
			ProviderConfig: map[string]interface{}{"host": "https://h"},
			ProviderAPIKey: "from-struct",
		},
	}
	if err := hydrateUsageTelemetryProviderConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.EffectiveUsageTelemetryProviderAPIKey() != "from-struct" {
		t.Fatal("expected ProviderAPIKey to win")
	}
}

func TestEffectiveUsageTelemetryProviderAPIKey_providerConfigApiKeyFallback(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		UsageTelemetry: UsageTelemetryConfig{
			ProviderConfig: map[string]interface{}{"host": "https://h", "api_key": "yaml-only"},
		},
	}
	if cfg.EffectiveUsageTelemetryProviderAPIKey() != "yaml-only" {
		t.Fatalf("got %q", cfg.EffectiveUsageTelemetryProviderAPIKey())
	}
}

// CRUISEKUBE_USAGETELEMETRY_PROVIDERAPIKEY binds to usageTelemetry.providerApiKey; keep this test so the env name stays aligned with Viper's nested key.
func TestViperBindsProviderApiKeyEnv(t *testing.T) {
	v := viper.New()
	v.SetEnvPrefix("cruisekube")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	t.Setenv("CRUISEKUBE_USAGETELEMETRY_PROVIDERAPIKEY", "probe")
	if got := v.GetString("usageTelemetry.providerApiKey"); got != "probe" {
		t.Fatalf("got %q", got)
	}
}
