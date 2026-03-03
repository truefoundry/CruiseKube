package cluster

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerScheduleTaskRejectsInvalidDuration(t *testing.T) {
	t.Parallel()

	scheduler := NewScheduler()

	err := scheduler.ScheduleTask(context.Background(), "task", "not-a-duration", func(context.Context) error {
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

	if err := scheduler.ScheduleTask(context.Background(), "task", "1h", func(context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("ScheduleTask() first call error = %v, want nil", err)
	}

	err := scheduler.ScheduleTask(context.Background(), "task", "1h", func(context.Context) error {
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

	err := scheduler.ScheduleTask(context.Background(), "task", "10ms", func(context.Context) error {
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
