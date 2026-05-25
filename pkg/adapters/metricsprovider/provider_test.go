package metricsprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/config"
)

func TestNewProviderConstructsPrometheusCompatibleProviders(t *testing.T) {
	testCases := []struct {
		name         string
		providerType config.MetricsProviderType
	}{
		{name: "prometheus", providerType: config.MetricsProviderTypePrometheus},
		{name: "kloudfuse", providerType: config.MetricsProviderTypeKloudfuse},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			provider, err := NewProvider(context.Background(), config.MetricsProviderConfig{
				Type: testCase.providerType,
				URL:  "http://example.com",
			})
			if err != nil {
				t.Fatalf("NewProvider() returned error: %v", err)
			}
			if provider == nil {
				t.Fatal("NewProvider() returned nil provider")
			}
			if got := provider.ProviderName(); got != string(testCase.providerType) {
				t.Fatalf("ProviderName() = %q, want %q", got, testCase.providerType)
			}
		})
	}
}

func TestNewProviderDefaultsToPrometheus(t *testing.T) {
	provider, err := NewProvider(context.Background(), config.MetricsProviderConfig{
		URL: "http://example.com",
	})
	if err != nil {
		t.Fatalf("NewProvider() returned error: %v", err)
	}
	if got := provider.ProviderName(); got != string(config.MetricsProviderTypePrometheus) {
		t.Fatalf("ProviderName() = %q, want %q", got, config.MetricsProviderTypePrometheus)
	}
}

func TestNewProviderAttachesBearerToken(t *testing.T) {
	const token = "factory-token"
	var gotAuthorization string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	provider, err := NewProvider(context.Background(), config.MetricsProviderConfig{
		Type:        config.MetricsProviderTypeKloudfuse,
		URL:         server.URL,
		BearerToken: token,
	})
	if err != nil {
		t.Fatalf("NewProvider() returned error: %v", err)
	}

	_, _, err = provider.ExecuteQueryWithRetry(context.Background(), "test-cluster", "up", "factory-token-test")
	if err != nil {
		t.Fatalf("ExecuteQueryWithRetry() returned error: %v", err)
	}

	if want := "Bearer " + token; gotAuthorization != want {
		t.Fatalf("Authorization header = %q, want %q", gotAuthorization, want)
	}
}
