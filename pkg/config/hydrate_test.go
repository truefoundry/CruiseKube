package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
)

func TestHydrateUsageTelemetryProviderConfig_providerApiKeyFromStruct(t *testing.T) {
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
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "api_key"); got != "key-from-struct" {
		t.Fatalf("api_key=%q", got)
	}
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "host"); got != "https://x.example" {
		t.Fatalf("host=%q", got)
	}
}

func TestHydrateUsageTelemetryProviderConfig_embedDefault(t *testing.T) {
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
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "api_key"); got != "baked-key" {
		t.Fatalf("api_key=%q", got)
	}
}

func TestHydrateUsageTelemetryProviderConfig_structWinsOverEmbed(t *testing.T) {
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
	if got := usageTelemetryStringFromMap(cfg.UsageTelemetry.ProviderConfig, "api_key"); got != "from-struct" {
		t.Fatalf("api_key=%q", got)
	}
}

// Helm sets CRUISEKUBE_USAGETELEMETRY_PROVIDERAPIKEY; keep this test so the env name stays aligned with Viper's nested key.
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
