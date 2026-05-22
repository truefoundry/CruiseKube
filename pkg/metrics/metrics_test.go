package metrics

import (
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/metrics/metricstest"
)

func TestRecordTaskRunObservesDurationAndUpdatesExistingMetrics(t *testing.T) {
	clusterID := metricstest.UniqueLabel(t, "cluster")
	taskName := metricstest.UniqueLabel(t, "task")
	status := StatusError
	source := TaskSourceManual
	duration := 1500 * time.Millisecond

	beforeCount := metricstest.SampleValue(t, "cruisekube_task_run_count", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    status,
	})

	RecordTaskRun(clusterID, taskName, status, source, duration)

	if got := metricstest.SampleValue(t, "cruisekube_task_duration_seconds_count", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    status,
		"source":    source,
	}); got != 1 {
		t.Fatalf("expected task duration histogram count 1, got %v", got)
	}

	if got := metricstest.SampleValue(t, "cruisekube_task_completion_time_seconds", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
	}); got != duration.Seconds() {
		t.Fatalf("expected task completion gauge %v, got %v", duration.Seconds(), got)
	}

	if got := metricstest.SampleValue(t, "cruisekube_task_run_count", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    status,
	}); got-beforeCount != 1 {
		t.Fatalf("expected task run counter to increase by 1, before=%v after=%v", beforeCount, got)
	}
}

func TestDurationHelpersObserveHistograms(t *testing.T) {
	clusterID := metricstest.UniqueLabel(t, "cluster")
	operation := metricstest.UniqueLabel(t, "operation")
	phase := metricstest.UniqueLabel(t, "phase")
	queryKind := metricstest.UniqueLabel(t, "query_kind")
	route := "/api/test/" + metricstest.UniqueLabel(t, "route")

	ObserveOperationDuration(clusterID, operation, StatusComplete, 2*time.Second)
	if got := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", map[string]string{
		"cluster":   clusterID,
		"operation": operation,
		"status":    StatusComplete,
	}); got != 1 {
		t.Fatalf("expected operation duration histogram count 1, got %v", got)
	}

	ObservePrometheusQueryDuration(clusterID, phase, queryKind, StatusSuccess, 3*time.Second)
	if got := metricstest.SampleValue(t, "cruisekube_prometheus_query_duration_seconds_count", map[string]string{
		"cluster":    clusterID,
		"phase":      phase,
		"query_kind": queryKind,
		"status":     StatusSuccess,
	}); got != 1 {
		t.Fatalf("expected prometheus query duration histogram count 1, got %v", got)
	}

	ObserveHTTPServerRequestDuration(route, "GET", 204, 100*time.Millisecond)
	if got := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  route,
		"method": "GET",
		"status": "204",
	}); got != 1 {
		t.Fatalf("expected http server request duration histogram count 1, got %v", got)
	}
}

func TestHelpersNormalizeEmptyLabels(t *testing.T) {
	beforeOperationCount := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", map[string]string{
		"cluster":   "unknown",
		"operation": "unknown",
		"status":    "unknown",
	})
	ObserveOperationDuration("", "", "", time.Second)
	if got := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", map[string]string{
		"cluster":   "unknown",
		"operation": "unknown",
		"status":    "unknown",
	}); got-beforeOperationCount != 1 {
		t.Fatalf("expected normalized operation duration count to increase by 1, before=%v after=%v", beforeOperationCount, got)
	}

	beforeHTTPCount := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  "unknown",
		"method": "unknown",
		"status": "503",
	})
	ObserveHTTPServerRequestDuration("", "", 503, time.Second)
	if got := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  "unknown",
		"method": "unknown",
		"status": "503",
	}); got-beforeHTTPCount != 1 {
		t.Fatalf("expected normalized http duration count to increase by 1, before=%v after=%v", beforeHTTPCount, got)
	}
}
