package posthog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/posthog/posthog-go"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/ports"
)

type providerConfig struct {
	APIKey string `json:"api_key"`
	Host   string `json:"host"`
}

// Reporter sends capture events via the official PostHog Go client.
type Reporter struct {
	// This is to prevent the race condition between the ReportHeartbeat and Close method.
	clientMu   sync.RWMutex
	client     posthog.Client
	distinctID string
}

const eventName = "cruisekube_cluster_heartbeat"

// NewReporterFromProviderConfig unmarshals providerConfig (api_key, optional host) and opens a PostHog client.
func NewReporterFromProviderConfig(distinctID string, providerConfigMap map[string]interface{}) (*Reporter, error) {
	if strings.TrimSpace(distinctID) == "" {
		return nil, fmt.Errorf("posthog reporter: distinct install id is empty")
	}
	if len(providerConfigMap) == 0 {
		return nil, fmt.Errorf("posthog reporter: providerConfig is empty")
	}
	raw, err := json.Marshal(providerConfigMap)

	if err != nil {
		return nil, fmt.Errorf("posthog reporter: marshal provider config: %w", err)
	}
	var cfg providerConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("posthog reporter: parse provider config: %w", err)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("posthog reporter: providerConfig.api_key is required")
	}
	endpoint := strings.TrimSpace(cfg.Host)
	if endpoint == "" {
		endpoint = posthog.DefaultEndpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")

	client, err := posthog.NewWithConfig(strings.TrimSpace(cfg.APIKey), posthog.Config{
		Endpoint:  endpoint,
		BatchSize: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("posthog reporter: init client: %w", err)
	}

	logging.Infof(context.Background(), "posthog reporter: initialized with distinct id %s", distinctID)
	return &Reporter{
		client:     client,
		distinctID: distinctID,
	}, nil
}

// ReportHeartbeat implements ports.UsageTelemetryReporter.
func (r *Reporter) ReportHeartbeat(ctx context.Context, hb ports.UsageHeartbeat) error {
	r.clientMu.RLock()
	defer r.clientMu.RUnlock()
	if r.client == nil {
		return fmt.Errorf("posthog reporter: client is closed or not initialized")
	}
	props := posthog.NewProperties().
		Set("k8s_major", hb.K8sMajor).
		Set("k8s_minor", hb.K8sMinor).
		Set("helm_chart_version", hb.HelmChartVersion)

	if err := r.client.Enqueue(posthog.Capture{
		DistinctId: r.distinctID,
		Event:      eventName,
		Properties: props,
	}); err != nil {
		return fmt.Errorf("posthog capture enqueue: %w", err)
	}
	return nil
}

// Close flushes pending events and shuts down the PostHog client.
func (r *Reporter) Close() error {
	r.clientMu.Lock()
	defer r.clientMu.Unlock()
	if r.client == nil {
		return nil
	}
	err := r.client.Close()
	r.client = nil
	if err != nil {
		return fmt.Errorf("posthog client close: %w", err)
	}
	return nil
}
