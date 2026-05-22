package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/metrics/metricstest"
)

func TestSchedulerScheduleTaskRejectsInvalidDuration(t *testing.T) {
	t.Parallel()

	scheduler := NewScheduler()

	err := scheduler.ScheduleTask(context.Background(), "task", "cluster", "not-a-duration", func(context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("ScheduleTask() error = nil, want invalid duration error")
	}
}

func TestSchedulerScheduleTaskRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	scheduler := NewScheduler()
	defer scheduler.Stop(context.Background())

	if err := scheduler.ScheduleTask(context.Background(), "task", "cluster", "1h", func(context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("ScheduleTask() first call error = %v, want nil", err)
	}

	err := scheduler.ScheduleTask(context.Background(), "task", "cluster", "1h", func(context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("ScheduleTask() duplicate error = nil, want duplicate task error")
	}
}

func TestSchedulerSkipsOverlappingExecutions(t *testing.T) {
	t.Parallel()

	scheduler := NewScheduler()
	t.Cleanup(func() {
		scheduler.Stop(context.Background())
	})

	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	unexpectedRun := make(chan struct{}, 1)
	var runCount atomic.Int32
	var firstRun atomic.Bool

	err := scheduler.ScheduleTask(context.Background(), "task", "cluster", "10ms", func(context.Context) error {
		runCount.Add(1)

		if firstRun.CompareAndSwap(false, true) {
			close(started)
			<-release
		} else {
			select {
			case unexpectedRun <- struct{}{}:
			default:
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("ScheduleTask() error = %v, want nil", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first task run")
	}

	select {
	case <-unexpectedRun:
		t.Fatal("task ran again while first execution was still blocked")
	case <-time.After(35 * time.Millisecond):
	}

	if got := runCount.Load(); got != 1 {
		t.Fatalf("run count while first execution is blocked = %d, want 1", got)
	}
}

func TestSchedulerRecordsTaskRunMetricsForSuccessAndError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		status string
		err    error
	}{
		{name: "success", status: "success"},
		{name: "error", status: "error", err: errors.New("boom")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheduler := NewScheduler()
			t.Cleanup(func() {
				scheduler.Stop(context.Background())
			})

			clusterID := uniqueSchedulerLabel(t, "cluster")
			taskName := uniqueSchedulerLabel(t, "task")
			labels := map[string]string{"cluster": clusterID, "task_name": taskName, "status": tc.status}
			before := metricstest.SampleValue(t, "cruisekube_task_run_count", labels)
			done := make(chan struct{})

			err := scheduler.ScheduleTask(context.Background(), taskName, clusterID, "1h", func(context.Context) error {
				close(done)
				return tc.err
			})
			if err != nil {
				t.Fatalf("ScheduleTask() error = %v, want nil", err)
			}

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for scheduled task run")
			}

			waitForCounter(t, labels, before+1)
		})
	}
}

func TestSchedulerExecuteTaskRecordsPanicStatus(t *testing.T) {
	t.Parallel()

	scheduler := NewScheduler()
	entry := &taskEntry{lock: make(chan struct{}, 1)}
	clusterID := uniqueSchedulerLabel(t, "cluster")
	taskName := uniqueSchedulerLabel(t, "task")
	labels := map[string]string{"cluster": clusterID, "task_name": taskName, "status": "panic"}
	before := metricstest.SampleValue(t, "cruisekube_task_run_count", labels)

	scheduler.executeTask(context.Background(), taskName, clusterID, entry, func(context.Context) error {
		panic("boom")
	})

	if got := metricstest.SampleValue(t, "cruisekube_task_run_count", labels); got-before != 1 {
		t.Fatalf("panic task counter increase = %v, want 1 (before=%v after=%v)", got-before, before, got)
	}
}

func TestSchedulerWaitReturnsAfterStop(t *testing.T) {
	t.Parallel()

	scheduler := NewScheduler()
	waitDone := make(chan struct{})

	go func() {
		scheduler.Wait(context.Background())
		close(waitDone)
	}()

	scheduler.Stop(context.Background())

	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("Wait() did not return after Stop()")
	}
}

func waitForCounter(t *testing.T, labels map[string]string, want float64) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for {
		if got := metricstest.SampleValue(t, "cruisekube_task_run_count", labels); got >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("counter did not reach %v", want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func uniqueSchedulerLabel(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s_%s_%d", prefix, t.Name(), time.Now().UnixNano())
}
