package posthog

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/ports"
)

func TestReporter_ReportHeartbeat(t *testing.T) {
	t.Parallel()
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/batch") {
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
		"host": srv.URL,
	}, "phc_test")
	if err != nil {
		t.Fatal(err)
	}

	hb := ports.UsageHeartbeat{
		CruisekubeVersion: "1.2.3",
		K8sMajor:          "1",
		K8sMinor:          "28",
		HelmChartVersion:  "0.9.0",
	}
	if err := r.ReportHeartbeat(context.Background(), hb); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	body := string(gotBody)
	if !strings.Contains(body, "phc_test") || !strings.Contains(body, eventName) || !strings.Contains(body, "install-xyz") {
		t.Fatalf("unexpected batch body: %s", body)
	}
	if !strings.Contains(body, "cruisekube_version") {
		t.Fatalf("missing properties in body: %s", body)
	}
}

func TestNewReporterFromProviderConfig_requiresAPIKey(t *testing.T) {
	t.Parallel()
	_, err := NewReporterFromProviderConfig("id", map[string]interface{}{"host": "https://example.com"}, "")
	if err == nil {
		t.Fatal("expected error")
	}
}
