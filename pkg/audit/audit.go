package audit

import (
	"context"
	"sync"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/types"
)

// Recorder is the singleton audit storage, set at startup (e.g. in main).
var Recorder *Audit

// Options holds optional configuration for the audit system.
type Options struct {
	BufferSize int
}

// Audit provides non-blocking recording of audit events to the database.
type Audit struct {
	db     ports.Database
	opts   Options
	ch     chan auditPayload
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

type auditPayload struct {
	clusterID string
	event     types.AuditEvent
}

const defaultBufferSize = 5000

// NewAudit creates an Audit that writes events asynchronously via the given database.
func NewAudit(ctx context.Context, db ports.Database, opts Options) *Audit {
	if opts.BufferSize <= 0 {
		opts.BufferSize = defaultBufferSize
	}
	a := &Audit{
		db:   db,
		opts: opts,
		ch:   make(chan auditPayload, opts.BufferSize),
		done: make(chan struct{}),
	}
	go a.run(ctx)
	return a
}

func (a *Audit) run(ctx context.Context) {
	for p := range a.ch {
		if err := a.db.InsertAuditEvent(p.clusterID, p.event); err != nil {
			logging.Errorf(ctx, "audit: failed to write event %s: %v", p.event.Category, err)
		}
	}
	close(a.done)
}

// Record enqueues an audit event for asynchronous write. Non-blocking; drops event if buffer full.
func (a *Audit) Record(ctx context.Context, clusterID string, event types.AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		logging.Warnf(ctx, "audit: Record called after Close, dropping event %s", event.Category)
		return
	}
	select {
	case a.ch <- auditPayload{clusterID: clusterID, event: event}:
	default:
		logging.Warnf(ctx, "audit: buffer full, dropping event %s", event.Category)
	}
}

// Close stops the audit worker and drains the queue. Idempotent.
func (a *Audit) Close() {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	close(a.ch)
	a.mu.Unlock()
	<-a.done
}
