package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/metrics"
)

func TestMetricsEndpointExposesTimingMetrics(t *testing.T) {
	clusterID := "default"
	taskName := "metrics_endpoint_task_" + uniqueMetricSuffix(t)
	operation := "metrics_endpoint_operation_" + uniqueMetricSuffix(t)
	queryKind := "metrics_endpoint_query_" + uniqueMetricSuffix(t)
	route := "/metrics-endpoint/" + uniqueMetricSuffix(t)

	metrics.RecordTaskRun(clusterID, taskName, "success", "manual", time.Second)
	metrics.ObserveOperationDuration(clusterID, operation, "complete", time.Second)
	metrics.ObservePrometheusQueryDuration(clusterID, "range_query", queryKind, "success", time.Second)
	metrics.ObserveHTTPServerRequestDuration(route, http.MethodGet, http.StatusAccepted, time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	SetupMetricsServerEngine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /metrics status %d, got %d", http.StatusOK, recorder.Code)
	}

	body := recorder.Body.String()
	assertMetricSampleLine(t, body, "cruisekube_task_duration_seconds_count", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    "success",
		"source":    "manual",
	})
	assertMetricSampleLine(t, body, "cruisekube_operation_duration_seconds_count", map[string]string{
		"cluster":   clusterID,
		"operation": operation,
		"status":    "complete",
	})
	assertMetricSampleLine(t, body, "cruisekube_prometheus_query_duration_seconds_count", map[string]string{
		"cluster":    clusterID,
		"phase":      "range_query",
		"query_kind": queryKind,
		"status":     "success",
	})
	assertMetricSampleLine(t, body, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  route,
		"method": http.MethodGet,
		"status": "202",
	})
}

func assertMetricSampleLine(t *testing.T, body, metricName string, labels map[string]string) {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, metricName+"{") && !strings.HasPrefix(line, metricName+" ") {
			continue
		}
		if metricLineHasLabels(line, labels) {
			return
		}
	}
	t.Fatalf("expected metrics body to contain %s with labels %v", metricName, labels)
}

func metricLineHasLabels(line string, labels map[string]string) bool {
	for name, value := range labels {
		if !strings.Contains(line, fmt.Sprintf("%s=%q", name, value)) {
			return false
		}
	}
	return true
}

func uniqueMetricSuffix(t *testing.T) string {
	t.Helper()
	return strings.NewReplacer("/", "_", " ", "_", "-", "_").Replace(t.Name()) + "_" + fmt.Sprint(time.Now().UnixNano())
}
