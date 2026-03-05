package main

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

const runtimeShutdownTimeout = 5 * time.Second

type runtimeManager struct {
	//nolint:containedctx // This coordinator owns the root cancellation state for all managed components.
	ctx      context.Context
	cancel   context.CancelFunc
	errCh    chan error
	mu       sync.Mutex
	cleanups []func(context.Context) error
}

func newRuntimeManager(parent context.Context) *runtimeManager {
	ctx, cancel := context.WithCancel(parent)

	return &runtimeManager{
		ctx:    ctx,
		cancel: cancel,
		errCh:  make(chan error, 1),
	}
}

func (m *runtimeManager) Go(name string, fn func(context.Context) error) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				m.reportError(fmt.Errorf("%s panic: %v\n%s", name, recovered, debug.Stack()))
			}
		}()

		if err := fn(m.ctx); err != nil {
			m.reportError(fmt.Errorf("%s: %w", name, err))
		}
	}()
}

func (m *runtimeManager) AddCleanup(fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanups = append(m.cleanups, fn)
}

func (m *runtimeManager) Wait() error {
	select {
	case err := <-m.errCh:
		return err
	case <-m.ctx.Done():
		return nil
	}
}

func (m *runtimeManager) Shutdown() error {
	m.cancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), runtimeShutdownTimeout)
	defer cancel()

	m.mu.Lock()
	cleanups := append([]func(context.Context) error(nil), m.cleanups...)
	m.mu.Unlock()

	var shutdownErr error
	for i := len(cleanups) - 1; i >= 0; i-- {
		if err := cleanups[i](shutdownCtx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	return shutdownErr
}

func (m *runtimeManager) reportError(err error) {
	select {
	case m.errCh <- err:
	default:
	}
}
