package cluster

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
)

type taskEntry struct {
	ticker *time.Ticker
	lock   chan struct{} // semaphore, size 1
}

type Scheduler struct {
	mu       sync.Mutex
	tasks    map[string]*taskEntry
	quit     chan struct{}
	stopOnce sync.Once
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks: make(map[string]*taskEntry),
		quit:  make(chan struct{}),
	}
}

func (s *Scheduler) ScheduleTask(
	ctx context.Context,
	name string,
	schedule string,
	task func(ctx context.Context) error,
) error {
	duration, err := time.ParseDuration(schedule)
	if err != nil {
		return fmt.Errorf("parse schedule for task %s: %w", name, err)
	}

	s.mu.Lock()
	if _, exists := s.tasks[name]; exists {
		s.mu.Unlock()
		return fmt.Errorf("task %s already exists", name)
	}

	entry := &taskEntry{
		ticker: time.NewTicker(duration),
		lock:   make(chan struct{}, 1), // semaphore
	}
	s.tasks[name] = entry
	s.mu.Unlock()

	go func() {
		// Run once immediately
		s.executeTask(ctx, name, entry, task)

		for {
			select {
			case <-entry.ticker.C:
				s.executeTask(ctx, name, entry, task)

			case <-s.quit:
				entry.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

func (s *Scheduler) executeTask(
	ctx context.Context,
	name string,
	entry *taskEntry,
	task func(ctx context.Context) error,
) {
	// Try to acquire semaphore
	select {
	case entry.lock <- struct{}{}:
		// acquired
	default:
		logging.Debugf(ctx, "Task %s is already running, skipping", name)
		return
	}

	// Install panic recovery before calling task to ensure semaphore is always released
	defer func() {
		if r := recover(); r != nil {
			logging.Errorf(ctx, "Task %s panicked: %v\nStack trace:\n%s", name, r, debug.Stack())
		}
	}()

	defer func() {
		<-entry.lock // release
	}()

	logging.Infof(ctx, "Launching task: %s", name)

	if err := task(ctx); err != nil {
		logging.Errorf(ctx, "Failed to run task %s: %v", name, err)
	}
}

func (s *Scheduler) Wait(ctx context.Context) {
	logging.Info(ctx, "Scheduler started")
	select {
	case <-s.quit:
	case <-ctx.Done():
	}
}

func (s *Scheduler) Stop(ctx context.Context) {
	logging.Info(ctx, "Stopping scheduler")
	s.stopOnce.Do(func() {
		close(s.quit)
	})
}
