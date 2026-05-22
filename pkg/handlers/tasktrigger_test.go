package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/metrics"
	"github.com/truefoundry/cruisekube/pkg/metrics/metricstest"
	"github.com/truefoundry/cruisekube/pkg/task"
)

func TestHandleTaskTriggerRecordsManualSuccessMetric(t *testing.T) {
	testHandleTaskTriggerRecordsManualMetric(t, nil, http.StatusOK, metrics.StatusSuccess)
}

func TestHandleTaskTriggerRecordsManualErrorMetric(t *testing.T) {
	testHandleTaskTriggerRecordsManualMetric(t, fmt.Errorf("task failed"), http.StatusInternalServerError, metrics.StatusError)
}

func TestHandleTaskTriggerRecordsManualPanicMetric(t *testing.T) {
	gin.SetMode(gin.TestMode)

	clusterID := "default"
	taskName := "manual_task_" + uniqueTaskTriggerMetricLabel(t)
	deps := HandlerDependencies{
		ClusterManager: &fakeTaskTriggerClusterManager{
			task: fakeTaskTriggerTask{
				name:      taskName,
				clusterID: clusterID,
				panics:    true,
			},
		},
	}
	labels := map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    metrics.StatusPanic,
		"source":    metrics.TaskSourceManual,
	}
	before := metricstest.SampleValue(t, "cruisekube_task_duration_seconds_count", labels)

	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/dev/clusters/:clusterID/tasks/:taskName/trigger", deps.HandleTaskTrigger)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/dev/clusters/"+clusterID+"/tasks/"+taskName+"/trigger", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	if got := metricstest.SampleValue(t, "cruisekube_task_duration_seconds_count", labels); got-before != 1 {
		t.Fatalf("expected manual panic duration count to increase by 1, before=%v after=%v", before, got)
	}
}

func testHandleTaskTriggerRecordsManualMetric(t *testing.T, runErr error, expectedStatus int, metricStatus string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	clusterID := "default"
	taskName := "manual_task_" + uniqueTaskTriggerMetricLabel(t)
	deps := HandlerDependencies{
		ClusterManager: &fakeTaskTriggerClusterManager{
			task: fakeTaskTriggerTask{
				name:      taskName,
				clusterID: clusterID,
				runErr:    runErr,
			},
		},
	}

	before := metricstest.SampleValue(t, "cruisekube_task_duration_seconds_count", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    metricStatus,
		"source":    metrics.TaskSourceManual,
	})

	router := gin.New()
	router.POST("/dev/clusters/:clusterID/tasks/:taskName/trigger", deps.HandleTaskTrigger)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/dev/clusters/"+clusterID+"/tasks/"+taskName+"/trigger", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, recorder.Code)
	}

	after := metricstest.SampleValue(t, "cruisekube_task_duration_seconds_count", map[string]string{
		"cluster":   clusterID,
		"task_name": taskName,
		"status":    metricStatus,
		"source":    metrics.TaskSourceManual,
	})
	if after-before != 1 {
		t.Fatalf("expected manual task duration count to increase by 1, before=%v after=%v", before, after)
	}
}

type fakeTaskTriggerTask struct {
	name      string
	clusterID string
	runErr    error
	panics    bool
}

func (f fakeTaskTriggerTask) GetName() string      { return f.name }
func (f fakeTaskTriggerTask) GetClusterID() string { return f.clusterID }
func (f fakeTaskTriggerTask) GetSchedule() string  { return "@every 1h" }
func (f fakeTaskTriggerTask) IsEnabled() bool      { return true }
func (f fakeTaskTriggerTask) GetCoreTask() any     { return nil }
func (f fakeTaskTriggerTask) Run(ctx context.Context) error {
	if f.panics {
		panic("task panicked")
	}
	return f.runErr
}

type fakeTaskTriggerClusterManager struct {
	task task.Task
}

func (f *fakeTaskTriggerClusterManager) RefreshClusters(ctx context.Context) error { return nil }
func (f *fakeTaskTriggerClusterManager) GetAllClusters() map[string]*cluster.ClusterClients {
	return nil
}
func (f *fakeTaskTriggerClusterManager) GetClusterIDs() []string { return []string{"default"} }
func (f *fakeTaskTriggerClusterManager) GetClusterClients(clusterID string) (*cluster.ClusterClients, error) {
	return nil, nil
}
func (f *fakeTaskTriggerClusterManager) GetPrometheusConnectionInfo(clusterID string) (*cluster.PrometheusConnectionInfo, error) {
	return nil, nil
}
func (f *fakeTaskTriggerClusterManager) GetClusterMode() cluster.ClusterMode {
	return cluster.ClusterModeSingle
}
func (f *fakeTaskTriggerClusterManager) AddTask(task task.Task) { f.task = task }
func (f *fakeTaskTriggerClusterManager) GetTask(taskName string) (task.Task, error) {
	if f.task == nil || f.task.GetName() != taskName {
		return nil, fmt.Errorf("task %s not found", taskName)
	}
	return f.task, nil
}
func (f *fakeTaskTriggerClusterManager) ScheduleAllTasks(ctx context.Context) error { return nil }
func (f *fakeTaskTriggerClusterManager) StopScheduler(ctx context.Context)          {}

func uniqueTaskTriggerMetricLabel(t *testing.T) string {
	t.Helper()
	return metricstest.UniqueLabel(t, "tasktrigger")
}
