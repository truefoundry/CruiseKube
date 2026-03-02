package main

import (
	"context"
	"errors"
	"slices"
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
