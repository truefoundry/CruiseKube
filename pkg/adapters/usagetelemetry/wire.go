// Package usageadapter wires the compiled-in usage telemetry reporter (directory name matches adapters layout).
package usageadapter

import (
	"fmt"

	"github.com/truefoundry/cruisekube/pkg/adapters/usagetelemetry/posthog"
	"github.com/truefoundry/cruisekube/pkg/ports"
)

// NewReporter returns the compiled-in usage telemetry reporter implementation.
func NewReporter(installID string, providerConfig map[string]interface{}, providerAPIKey string) (ports.UsageTelemetryReporter, error) {
	if providerConfig == nil {
		providerConfig = map[string]interface{}{}
	}
	rep, err := posthog.NewReporterFromProviderConfig(installID, providerConfig, providerAPIKey)
	if err != nil {
		return nil, fmt.Errorf("usage telemetry reporter: %w", err)
	}
	return rep, nil
}
