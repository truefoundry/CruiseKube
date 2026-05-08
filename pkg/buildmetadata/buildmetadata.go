// Package buildmetadata holds link-time build metadata (set via -ldflags).
package buildmetadata

// Version is the release identifier served by the API (e.g. stats endpoint).
var Version = "dev"

// DefaultUsageTelemetryProviderAPIKey is set at link time via -ldflags -X when building release images (Docker ARG USAGETELEMETRY_PROVIDER_API_KEY).
var DefaultUsageTelemetryProviderAPIKey string
