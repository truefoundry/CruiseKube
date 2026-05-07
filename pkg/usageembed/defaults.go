// Package usageembed holds link-time defaults for usage telemetry.
package usageembed

// DefaultUsageTelemetryProviderAPIKey is set at link time via -ldflags -X when building release images (Docker ARG USAGETELEMETRY_PROVIDER_API_KEY).
var DefaultUsageTelemetryProviderAPIKey string
