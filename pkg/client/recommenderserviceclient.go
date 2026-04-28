package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/contextutils"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/metrics"
	"github.com/truefoundry/cruisekube/pkg/types"
	admissionv1 "k8s.io/api/admission/v1"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TODO: This client should be getting generated and not hardcoded
type RecommenderServiceClient struct {
	host          string
	httpClient    *http.Client
	basicUser     string
	basicPassword string
	clusterToken  string
}

type ClientConfig struct {
	Host         string
	Username     string
	Password     string
	ClusterToken string
	Timeout      time.Duration
}

// MutatingPatchRequest is the request body for mutating patch
// Manifest is the complete incoming object; the controller handles Pod and PDB separately.
type MutatingPatchRequest struct {
	Review admissionv1.AdmissionReview `json:"review"`
}

// JSONPatchOp is a single RFC 6902 JSON Patch operation.
// Value is optional and omitted when op is "remove".
type JSONPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewRecommenderServiceClient(config ClientConfig) *RecommenderServiceClient {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &RecommenderServiceClient{
		host: strings.TrimSuffix(config.Host, "/"),
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   timeout,
		},
		basicUser:     config.Username,
		basicPassword: config.Password,
		clusterToken:  config.ClusterToken,
	}
}

func NewRecommenderServiceClientWithClusterToken(host, clusterToken string) *RecommenderServiceClient {
	return NewRecommenderServiceClient(ClientConfig{
		Host:         host,
		ClusterToken: clusterToken,
	})
}

func (c *RecommenderServiceClient) makeRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	var err error
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		if clusterId, ok := contextutils.GetCluster(ctx); ok {
			metrics.WebhookControllerAPICallsTotal.WithLabelValues(clusterId, status, endpoint).Inc()
		}
	}()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := c.host + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.basicUser != "" || c.basicPassword != "" {
		req.SetBasicAuth(c.basicUser, c.basicPassword)
	} else if c.clusterToken != "" {
		logging.Infof(ctx, "Setting cluster token")
		req.Header.Set("x-cluster-token", c.clusterToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			logging.Errorf(ctx, "Failed to close response body: %v", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func (c *RecommenderServiceClient) GetClusterStats(ctx context.Context, clusterID string) (*types.StatsResponse, error) {
	var result types.StatsResponse
	endpoint := fmt.Sprintf("/api/v1/clusters/%s/stats", clusterID)
	err := c.makeRequest(ctx, "GET", endpoint, nil, &result)
	return &result, err
}

// WebhookMutatingPatch POSTs the given body to the controller's mutatingPatch endpoint and returns the response body (JSON patch array).
// On non-2xx or error, returns an error. Caller should treat error as "return empty patches".
func (c *RecommenderServiceClient) WebhookMutatingPatch(ctx context.Context, clusterID string, body MutatingPatchRequest) ([]JSONPatchOp, error) {
	endpoint := fmt.Sprintf("/api/v1/webhook/clusters/%s/mutate", clusterID)
	var result []JSONPatchOp
	err := c.makeRequest(ctx, "POST", endpoint, body, &result)
	return result, err
}
