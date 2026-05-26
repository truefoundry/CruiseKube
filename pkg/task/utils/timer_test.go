package utils

import (
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/metrics"
	"github.com/truefoundry/cruisekube/pkg/metrics/metricstest"
)

func TestStartTimedOperationRecordsOperationDurationWithStatus(t *testing.T) {
	clusterID := metricstest.UniqueLabel(t, "cluster")
	operation := metricstest.UniqueLabel(t, "operation")
	labels := map[string]string{
		"cluster":   clusterID,
		"operation": operation,
		"status":    metrics.StatusError,
	}
	before := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", labels)

	finish := StartTimedOperation(contextutils.WithCluster(t.Context(), clusterID), operation)
	time.Sleep(time.Millisecond)
	finish(metrics.StatusError)

	if got := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", labels); got-before != 1 {
		t.Fatalf("operation duration sample count increase = %v, want 1 (before=%v after=%v)", got-before, before, got)
	}
}

func TestStartTimedOperationDefaultsEmptyStatusToSuccess(t *testing.T) {
	clusterID := metricstest.UniqueLabel(t, "cluster")
	operation := metricstest.UniqueLabel(t, "operation")
	labels := map[string]string{
		"cluster":   clusterID,
		"operation": operation,
		"status":    metrics.StatusSuccess,
	}
	before := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", labels)

	finish := StartTimedOperation(contextutils.WithCluster(t.Context(), clusterID), operation)
	finish("")

	if got := metricstest.SampleValue(t, "cruisekube_operation_duration_seconds_count", labels); got-before != 1 {
		t.Fatalf("operation duration sample count increase = %v, want 1 (before=%v after=%v)", got-before, before, got)
	}
}
