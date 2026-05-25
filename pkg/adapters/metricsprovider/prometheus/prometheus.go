package prometheus

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/redaction"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type PrometheusClientConfig struct {
	// For Client
	PrometheusURL       string
	ProviderName        string
	BearerToken         string
	QueryTimeout        time.Duration
	MaxConnsPerHost     int
	MaxIdleConns        int
	IdleConnTimeout     time.Duration
	ResponseTimeout     time.Duration
	DialTimeout         time.Duration
	KeepAlive           time.Duration
	TLSHandshakeTimeout time.Duration
	// InsecureSkipVerify disables TLS certificate verification, which can expose
	// connections to MITM attacks. Avoid this in production and prefer valid
	// certificates or other secure alternatives when possible.
	InsecureSkipVerify bool

	// For Provider
	MaxQueryRetries      int
	RetryBackoffBase     time.Duration
	MaxConcurrentQueries int
}

type PrometheusProvider struct {
	client          v1.API
	config          *PrometheusClientConfig
	querySemaphores sync.Map
}

func NewPrometheusProvider(ctx context.Context, config *PrometheusClientConfig) (*PrometheusProvider, error) {
	if config.ProviderName == "" {
		config.ProviderName = "prometheus"
	}

	// Create optimized HTTP transport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: config.ResponseTimeout,
		DisableCompression:    false, // Enable compression for better performance
	}
	if config.InsecureSkipVerify {
		// #nosec G402 -- opt-in for environments using self-signed certs in CI/dev.
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}
	}

	// Wrap transport with bearer token if provided
	var finalTransport http.RoundTripper = transport
	if config.BearerToken != "" {
		finalTransport = &BearerTokenRoundTripper{
			BearerToken: config.BearerToken,
			Proxied:     transport,
		}
	}

	// Create optimized HTTP client
	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(finalTransport),
		Timeout:   config.ResponseTimeout,
	}

	// Create Prometheus API client
	apiClient, err := api.NewClient(api.Config{
		Address: config.PrometheusURL,
		Client:  httpClient,
	})
	if err != nil {
		logging.Error(ctx, fmt.Sprintf("Failed to create %s metrics provider client", config.ProviderName), err)
		return nil, fmt.Errorf("failed to create %s metrics provider client: %w", config.ProviderName, err)
	}
	client := v1.API(v1.NewAPI(apiClient))
	if config.BearerToken != "" {
		client = redactingAPI{API: client, bearerToken: config.BearerToken}
	}

	logging.Infof(ctx, "%s metrics provider client initialized with URL: %s", config.ProviderName, config.PrometheusURL)
	logging.Infof(ctx, "  - Query timeout: %v", config.QueryTimeout)
	logging.Infof(ctx, "  - Max connections per host: %d", config.MaxConnsPerHost)
	logging.Infof(ctx, "  - Max idle connections: %d", config.MaxIdleConns)
	logging.Infof(ctx, "  - Idle connection timeout: %v", config.IdleConnTimeout)
	logging.Infof(ctx, "  - Insecure TLS skip verify: %t", config.InsecureSkipVerify)

	return &PrometheusProvider{
		client: client,
		config: config,
	}, nil
}

func (p *PrometheusProvider) GetClient() v1.API {
	return p.client
}

func (p *PrometheusProvider) ProviderName() string {
	if p == nil || p.config == nil || p.config.ProviderName == "" {
		return "prometheus"
	}
	return p.config.ProviderName
}

func (p *PrometheusProvider) redactError(err error) error {
	if p == nil || p.config == nil {
		return err
	}
	return redactErrorWithToken(err, p.config.BearerToken)
}

func redactErrorWithToken(err error, bearerToken string) error {
	return redaction.Error(err, bearerToken)
}

type redactingAPI struct {
	v1.API
	bearerToken string
}

func (api redactingAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	value, warnings, err := api.API.Query(ctx, query, ts, opts...)
	return value, warnings, redactErrorWithToken(err, api.bearerToken)
}

func (api redactingAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	value, warnings, err := api.API.QueryRange(ctx, query, r, opts...)
	return value, warnings, redactErrorWithToken(err, api.bearerToken)
}

func (api redactingAPI) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	buildInfo, err := api.API.Buildinfo(ctx)
	return buildInfo, redactErrorWithToken(err, api.bearerToken)
}
