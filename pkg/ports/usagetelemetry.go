package ports

import "context"

// UsageHeartbeat is anonymous cluster-level metadata for a usage ping (no workload or customer identifiers).
type UsageHeartbeat struct {
	CruisekubeVersion  string
	NodeTotal          int
	NodeReady          int
	K8sMajor           string
	K8sMinor           string
	ControllerMode     string
	TargetNamespaceSet bool
	HelmChartVersion   string
}

// UsageTelemetryReporter sends a heartbeat to the configured product analytics backend.
type UsageTelemetryReporter interface {
	ReportHeartbeat(ctx context.Context, hb UsageHeartbeat) error
}
