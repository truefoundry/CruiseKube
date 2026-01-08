package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
)

type Scheduler struct {
	mu           sync.RWMutex
	tasks        map[string]*time.Ticker
	runningTasks map[string]bool
	quit         chan struct{}
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:        make(map[string]*time.Ticker),
		runningTasks: make(map[string]bool),
		quit:         make(chan struct{}),
	}
}

func (s *Scheduler) ScheduleTask(ctx context.Context, name string, schedule string, task func(ctx context.Context) error) {
	duration, err := time.ParseDuration(schedule)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse schedule for task %s: %v", name, err)
		return
	}

	ticker := time.NewTicker(duration)
	s.tasks[name] = ticker

	go func() {
		s.executeTask(ctx, name, task)

		for {
			select {
			case <-ticker.C:
				s.executeTask(ctx, name, task)
			case <-s.quit:
				ticker.Stop()
				return
			}
		}
	}()
}

// executeTask safely executes a task with deduplication to prevent concurrent runs
func (s *Scheduler) executeTask(ctx context.Context, name string, task func(ctx context.Context) error) {
	s.mu.Lock()
	if s.runningTasks[name] {
		logging.Infof(ctx, "Task %s is already running, skipping execution", name)
		s.mu.Unlock()
		return
	}
	// Mark the task as running
	s.runningTasks[name] = true
	s.mu.Unlock()

	logging.Infof(ctx, "Launching task: %s", name)

	// Execute the task and ensure cleanup happens regardless of success/failure
	defer func() {
		s.mu.Lock()
		delete(s.runningTasks, name)
		s.mu.Unlock()
	}()

	if err := task(ctx); err != nil {
		logging.Errorf(ctx, "Failed to run task %s: %v", name, err)
	}
}

func (s *Scheduler) Wait(ctx context.Context) {
	logging.Info(ctx, "Scheduler started")
	<-s.quit
}

func (s *Scheduler) Stop(ctx context.Context) {
	logging.Info(ctx, "Stopping scheduler")
	close(s.quit)
}
