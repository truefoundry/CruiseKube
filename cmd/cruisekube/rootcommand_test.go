package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/truefoundry/cruisekube/pkg/config"
)

func newRootCommandForTest(t *testing.T) *cobra.Command {
	t.Helper()

	oldV := v
	oldConfigFilePath := configFilePath
	v = viper.New()
	configFilePath = ""
	t.Cleanup(func() {
		v = oldV
		configFilePath = oldConfigFilePath
	})

	return newRootCommand(context.Background())
}

func TestRootCommandProviderOverrideFlagsBindBothDependencyBlocks(t *testing.T) {
	cmd := newRootCommandForTest(t)

	if cmd.PersistentFlags().Lookup("metrics-provider-bearer-token") != nil {
		t.Fatal("did not expect bearer-token CLI flag")
	}
	if err := cmd.PersistentFlags().Set("metrics-provider", "kloudfuse"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.PersistentFlags().Set("metrics-provider-url", "https://kloudfuse.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.PersistentFlags().Set("metrics-provider-insecure-skip-tls-verify", "true"); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"dependencies.local.metricsProvider.type",
		"dependencies.inCluster.metricsProvider.type",
	} {
		if got := v.GetString(key); got != "kloudfuse" {
			t.Fatalf("%s = %q, want kloudfuse", key, got)
		}
	}
	for _, key := range []string{
		"dependencies.local.metricsProvider.url",
		"dependencies.inCluster.metricsProvider.url",
	} {
		if got := v.GetString(key); got != "https://kloudfuse.example.com" {
			t.Fatalf("%s = %q, want https://kloudfuse.example.com", key, got)
		}
	}
	for _, key := range []string{
		"dependencies.local.metricsProvider.insecureSkipTLSVerify",
		"dependencies.inCluster.metricsProvider.insecureSkipTLSVerify",
	} {
		if got := v.GetBool(key); !got {
			t.Fatalf("%s = %t, want true", key, got)
		}
	}
}

func TestRootCommandProviderOverrideEnvBindsBothDependencyBlocks(t *testing.T) {
	t.Setenv("CRUISEKUBE_METRICS_PROVIDER", "kloudfuse")
	t.Setenv("CRUISEKUBE_METRICS_PROVIDER_URL", "https://kloudfuse.example.com")
	t.Setenv("CRUISEKUBE_METRICS_PROVIDER_INSECURE_SKIP_TLS_VERIFY", "true")
	newRootCommandForTest(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithViperInstance(context.Background(), v, configPath)
	if err != nil {
		t.Fatal(err)
	}

	for name, provider := range map[string]config.MetricsProviderConfig{
		"local":     cfg.Dependencies.Local.MetricsProvider,
		"inCluster": cfg.Dependencies.InCluster.MetricsProvider,
	} {
		if provider.Type != config.MetricsProviderTypeKloudfuse {
			t.Fatalf("%s type = %q, want kloudfuse", name, provider.Type)
		}
		if provider.URL != "https://kloudfuse.example.com" {
			t.Fatalf("%s url = %q, want https://kloudfuse.example.com", name, provider.URL)
		}
		if !provider.InsecureSkipTLSVerify {
			t.Fatalf("%s insecureSkipTLSVerify = false, want true", name)
		}
	}
}
