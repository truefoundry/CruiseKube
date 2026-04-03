package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestRuntimeManagerWaitReturnsComponentError(t *testing.T) {
	t.Parallel()

	manager := newRuntimeManager(context.Background())
	expectedErr := errors.New("boom")

	manager.Go("metrics", func(context.Context) error {
		return expectedErr
	})

	err := manager.Wait()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Wait() error = %v, want wrapped %v", err, expectedErr)
	}
}

func TestRuntimeManagerShutdownRunsCleanupsInReverseOrder(t *testing.T) {
	t.Parallel()

	manager := newRuntimeManager(context.Background())
	var order []string

	manager.AddCleanup(func(context.Context) error {
		order = append(order, "first")
		return nil
	})
	manager.AddCleanup(func(context.Context) error {
		order = append(order, "second")
		return nil
	})

	if err := manager.Shutdown(); err != nil {
		t.Fatalf("Shutdown() error = %v, want nil", err)
	}

	expectedOrder := []string{"second", "first"}
	if !slices.Equal(order, expectedOrder) {
		t.Fatalf("cleanup order = %v, want %v", order, expectedOrder)
	}
}

func TestRuntimeManagerWaitReturnsNilWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	manager := newRuntimeManager(ctx)

	cancel()

	if err := manager.Wait(); err != nil {
		t.Fatalf("Wait() error = %v, want nil", err)
	}
}

func TestRuntimeManagerShutdownAggregatesCleanupErrors(t *testing.T) {
	t.Parallel()

	manager := newRuntimeManager(context.Background())
	firstErr := errors.New("first cleanup")
	secondErr := errors.New("second cleanup")

	manager.AddCleanup(func(context.Context) error {
		return firstErr
	})
	manager.AddCleanup(func(context.Context) error {
		return secondErr
	})

	err := manager.Shutdown()
	if err == nil {
		t.Fatal("Shutdown() error = nil, want aggregated error")
	}
	if !errors.Is(err, firstErr) {
		t.Fatalf("Shutdown() error = %v, want wrapped %v", err, firstErr)
	}
	if !errors.Is(err, secondErr) {
		t.Fatalf("Shutdown() error = %v, want wrapped %v", err, secondErr)
	}
}

func TestRuntimeManagerGoRecoversPanics(t *testing.T) {
	t.Parallel()

	manager := newRuntimeManager(context.Background())

	manager.Go("worker", func(context.Context) error {
		panic("boom")
	})

	err := manager.Wait()
	if err == nil {
		t.Fatal("Wait() error = nil, want panic error")
	}
	if !strings.Contains(err.Error(), "worker panic: boom") {
		t.Fatalf("Wait() error = %v, want panic details", err)
	}
}
