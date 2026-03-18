package prometheus

import "testing"

func TestGetPrometheusClientConfigPropagatesInsecureSkipVerify(t *testing.T) {
	cfg := GetPrometheusClientConfig("https://prometheus.example.com", true)

	if cfg.PrometheusURL != "https://prometheus.example.com" {
		t.Fatalf("expected prometheus URL to be propagated, got %q", cfg.PrometheusURL)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("expected insecure TLS skip verify to be enabled")
	}
}
