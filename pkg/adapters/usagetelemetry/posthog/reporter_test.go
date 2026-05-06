package posthog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/ports"
)

func TestReporter_ReportHeartbeat(t *testing.T) {
	t.Parallel()
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/capture/" {
			t.Errorf("path %q", r.URL.Path)
		}
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	r, err := NewReporterFromProviderConfig("install-xyz", map[string]interface{}{
		"api_key":  "phc_test",
		"api_host": srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	hb := ports.UsageHeartbeat{
		CruisekubeVersion:  "1.2.3",
		NodeTotal:          2,
		NodeReady:          1,
		K8sMajor:           "1",
		K8sMinor:           "28",
		ControllerMode:     "in-cluster",
		TargetNamespaceSet: false,
		HelmChartVersion:   "0.9.0",
	}
	if err := r.ReportHeartbeat(context.Background(), hb); err != nil {
		t.Fatal(err)
	}
	var payload capturePayload
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(gotBody))
	}
	if payload.APIKey != "phc_test" || payload.Event != eventName || payload.DistinctID != "install-xyz" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Properties["cruisekube_version"] != "1.2.3" || payload.Properties["node_total"] != float64(2) {
		t.Fatalf("properties: %#v", payload.Properties)
	}
}

func TestNewReporterFromProviderConfig_requiresAPIKey(t *testing.T) {
	t.Parallel()
	_, err := NewReporterFromProviderConfig("id", map[string]interface{}{"api_host": "https://example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}
