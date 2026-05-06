package posthog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/ports"
)

const defaultAPIHost = "https://us.i.posthog.com"

type reporterConfig struct {
	APIKey     string `json:"api_key"`
	APIHost    string `json:"api_host"`
	HTTPClient *http.Client
}

// Reporter sends capture events to PostHog's HTTP API.
type Reporter struct {
	cfg         reporterConfig
	distinctID  string
	capturePath string
	httpClient  *http.Client
}

// NewReporterFromProviderConfig unmarshals the opaque providerConfig map for PostHog (api_key, optional api_host).
func NewReporterFromProviderConfig(distinctID string, providerConfig map[string]interface{}) (*Reporter, error) {
	if strings.TrimSpace(distinctID) == "" {
		return nil, fmt.Errorf("posthog reporter: distinct install id is empty")
	}
	raw, err := json.Marshal(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("posthog reporter: marshal provider config: %w", err)
	}
	var parsed reporterConfig
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("posthog reporter: parse provider config: %w", err)
	}
	if strings.TrimSpace(parsed.APIKey) == "" {
		return nil, fmt.Errorf("posthog reporter: providerConfig.api_key is required")
	}
	host := strings.TrimSpace(parsed.APIHost)
	if host == "" {
		host = defaultAPIHost
	}
	host = strings.TrimRight(host, "/")
	client := parsed.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Reporter{
		cfg:         parsed,
		distinctID:  distinctID,
		capturePath: host + "/capture/",
		httpClient:  client,
	}, nil
}

type capturePayload struct {
	APIKey     string                 `json:"api_key"`
	Event      string                 `json:"event"`
	DistinctID string                 `json:"distinct_id"`
	Properties map[string]interface{} `json:"properties"`
	Library    string                 `json:"library,omitempty"`
	LibraryVer string                 `json:"lib_version,omitempty"`
}

const eventName = "cruisekube_cluster_heartbeat"

// ReportHeartbeat implements ports.UsageTelemetryReporter.
func (r *Reporter) ReportHeartbeat(ctx context.Context, hb ports.UsageHeartbeat) error {
	props := map[string]interface{}{
		"cruisekube_version":   hb.CruisekubeVersion,
		"node_total":           hb.NodeTotal,
		"node_ready":           hb.NodeReady,
		"k8s_major":            hb.K8sMajor,
		"k8s_minor":            hb.K8sMinor,
		"controller_mode":      hb.ControllerMode,
		"target_namespace_set": hb.TargetNamespaceSet,
		"helm_chart_version":   hb.HelmChartVersion,
	}
	body, err := json.Marshal(capturePayload{
		APIKey:     r.cfg.APIKey,
		Event:      eventName,
		DistinctID: r.distinctID,
		Properties: props,
		Library:    "cruisekube",
	})
	if err != nil {
		return fmt.Errorf("posthog capture marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.capturePath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("posthog capture request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("posthog capture http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("posthog capture: status %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
