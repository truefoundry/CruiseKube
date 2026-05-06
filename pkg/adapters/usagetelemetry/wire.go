// Package usageadapter wires the compiled-in usage telemetry reporter (directory name matches adapters layout).
package usageadapter

import (
	"github.com/truefoundry/cruisekube/pkg/adapters/usagetelemetry/posthog"
	"github.com/truefoundry/cruisekube/pkg/ports"
)

// NewReporter returns the compiled-in usage telemetry reporter implementation.
func NewReporter(installID string, providerConfig map[string]interface{}) (ports.UsageTelemetryReporter, error) {
	if providerConfig == nil {
		providerConfig = map[string]interface{}{}
	}
	return posthog.NewReporterFromProviderConfig(installID, providerConfig)
}
