package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestPrometheusProviderSendsBearerToken(t *testing.T) {
	const token = "test-token"
	var gotAuthorization string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	provider, err := NewPrometheusProvider(context.Background(), testPrometheusClientConfig(server.URL, token))
	if err != nil {
		t.Fatalf("NewPrometheusProvider() returned error: %v", err)
	}

	_, _, err = provider.ExecuteQueryWithRetry(context.Background(), "test-cluster", "up", "bearer-token-test")
	if err != nil {
		t.Fatalf("ExecuteQueryWithRetry() returned error: %v", err)
	}

	if want := "Bearer " + token; gotAuthorization != want {
		t.Fatalf("Authorization header = %q, want %q", gotAuthorization, want)
	}
}

func TestPrometheusProviderRedactsBearerTokenFromErrors(t *testing.T) {
	const token = "secret-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"status":"error","errorType":"bad_data","error":"upstream error included token %s"}`, token)
	}))
	defer server.Close()

	provider, err := NewPrometheusProvider(context.Background(), testPrometheusClientConfig(server.URL, token))
	if err != nil {
		t.Fatalf("NewPrometheusProvider() returned error: %v", err)
	}

	_, _, err = provider.ExecuteQueryWithRetry(context.Background(), "test-cluster", "up", "redaction-test")
	if err == nil {
		t.Fatal("ExecuteQueryWithRetry() returned nil error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked bearer token: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error did not include redaction marker: %v", err)
	}
}

func TestPrometheusProviderRedactsBearerTokenFromClientQueryRangeErrors(t *testing.T) {
	const token = "secret-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"status":"error","errorType":"bad_data","error":"query_range echoed token %s"}`, token)
	}))
	defer server.Close()

	provider, err := NewPrometheusProvider(context.Background(), testPrometheusClientConfig(server.URL, token))
	if err != nil {
		t.Fatalf("NewPrometheusProvider() returned error: %v", err)
	}

	_, _, err = provider.GetClient().QueryRange(context.Background(), "up", promv1.Range{
		Start: time.Now().Add(-time.Minute),
		End:   time.Now(),
		Step:  time.Minute,
	})
	if err == nil {
		t.Fatal("QueryRange() returned nil error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked bearer token: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error did not include redaction marker: %v", err)
	}
}

func TestPrometheusProviderRedactsBearerTokenFromClientBuildinfoErrors(t *testing.T) {
	const token = "secret-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"status":"error","errorType":"bad_data","error":"buildinfo echoed token %s"}`, token)
	}))
	defer server.Close()

	provider, err := NewPrometheusProvider(context.Background(), testPrometheusClientConfig(server.URL, token))
	if err != nil {
		t.Fatalf("NewPrometheusProvider() returned error: %v", err)
	}

	_, err = provider.GetClient().Buildinfo(context.Background())
	if err == nil {
		t.Fatal("Buildinfo() returned nil error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked bearer token: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error did not include redaction marker: %v", err)
	}
}

func testPrometheusClientConfig(serverURL, token string) *PrometheusClientConfig {
	cfg := GetPrometheusClientConfig(serverURL, false)
	cfg.ProviderName = "kloudfuse"
	cfg.BearerToken = token
	cfg.QueryTimeout = time.Second
	cfg.ResponseTimeout = time.Second
	cfg.MaxQueryRetries = 1
	cfg.RetryBackoffBase = time.Millisecond
	return cfg
}
