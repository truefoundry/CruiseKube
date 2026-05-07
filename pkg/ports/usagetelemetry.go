package ports

import "context"

// UsageHeartbeat is anonymous cluster-level metadata for a usage ping (no workload or customer identifiers).
type UsageHeartbeat struct {
	K8sMajor         string
	K8sMinor         string
	HelmChartVersion string
}

// UsageTelemetryReporter sends a heartbeat to the configured product analytics backend.
type UsageTelemetryReporter interface {
	ReportHeartbeat(ctx context.Context, hb UsageHeartbeat) error
	Close() error
}
