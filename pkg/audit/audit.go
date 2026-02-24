package audit

import (
	"context"
	"sync"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/types"
)

// Stg is the singleton audit storage, set at startup (e.g. in main).
var Stg *Audit

// Options holds optional configuration for the audit system.
type Options struct {
	// BufferSize is the capacity of the async write channel (default 1000).
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

const defaultBufferSize = 1000

// NewAudit creates an Audit that writes events asynchronously via the given database.
func NewAudit(db ports.Database, opts Options) *Audit {
	if opts.BufferSize <= 0 {
		opts.BufferSize = defaultBufferSize
	}
	a := &Audit{
		db:   db,
		opts: opts,
		ch:   make(chan auditPayload, opts.BufferSize),
		done: make(chan struct{}),
	}
	go a.run()
	return a
}

func (a *Audit) run() {
	for p := range a.ch {
		evt := p.event
		if evt.Timestamp.IsZero() {
			evt.Timestamp = time.Now().UTC()
		} else {
			evt.Timestamp = evt.Timestamp.UTC()
		}
		if err := a.db.InsertAuditEvent(p.clusterID, evt); err != nil {
			logging.Errorf(context.Background(), "audit: failed to write event %s: %v", evt.Category, err)
		}
	}
	close(a.done)
}

// Record enqueues an audit event for asynchronous write. Non-blocking; drops event if buffer full.
// Timestamp is set to UTC now if zero. Do not modify the event after calling Record.
func (a *Audit) Record(ctx context.Context, clusterID string, event types.AuditEvent) {
	if a == nil {
		return
	}
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
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
	a.mu.Unlock()
	close(a.ch)
	<-a.done
}
