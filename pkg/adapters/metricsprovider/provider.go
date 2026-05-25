package metricsprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/truefoundry/cruisekube/pkg/adapters/metricsprovider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/config"
)

// NewProvider constructs the configured metrics provider. Prometheus and
// Kloudfuse both expose Prometheus-compatible APIs, so they share the existing
// PromQL client implementation.
func NewProvider(ctx context.Context, providerConfig config.MetricsProviderConfig) (*prometheus.PrometheusProvider, error) {
	providerName := strings.TrimSpace(string(providerConfig.Type))
	if providerName == "" {
		providerName = string(config.MetricsProviderTypePrometheus)
	}

	switch config.MetricsProviderType(providerName) {
	case config.MetricsProviderTypePrometheus, config.MetricsProviderTypeKloudfuse:
		clientConfig := prometheus.GetPrometheusClientConfig(
			strings.TrimSpace(providerConfig.URL),
			providerConfig.InsecureSkipTLSVerify,
		)
		clientConfig.ProviderName = providerName
		clientConfig.BearerToken = strings.TrimSpace(providerConfig.BearerToken)

		provider, err := prometheus.NewPrometheusProvider(ctx, clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize %s metrics provider: %w", providerName, err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported metrics provider type %q", providerName)
	}
}
