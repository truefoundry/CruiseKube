package types

import "time"

// EventType is the severity/type of the audit event (Warning, Error, Normal, Fatal, Info).
type EventType string

const (
	EventTypeNormal  EventType = "Normal"
	EventTypeWarning EventType = "Warning"
	EventTypeError   EventType = "Error"
	EventTypeFatal   EventType = "Fatal"
	EventTypeInfo    EventType = "Info"
)

const (
	AuditDetailCPURequestMillis = "cpuRequestMillis"
	AuditDetailCPULimitMillis   = "cpuLimitMillis"
	AuditDetailMemoryRequestMB  = "memoryRequestMB"
	AuditDetailMemoryLimitMB    = "memoryLimitMB"
)

// EventCategory is the category/name of the audit event per the CruiseKube Audit spec.
type EventCategory string

const (
	EventCategoryCPURecommendationApplied    EventCategory = "CPU_RECOMMENDATION_APPLIED"
	EventCategoryMemoryRecommendationApplied EventCategory = "MEMORY_RECOMMENDATION_APPLIED"
	EventCategoryPODDisruptionBlockRemoved   EventCategory = "POD_DISRUPTION_BLOCK_REMOVED"
	EventCategoryPODDisruptionBlockRestored  EventCategory = "POD_DISRUPTION_BLOCK_RESTORED"
	EventCategoryPDBRelaxed                  EventCategory = "PDB_RELAXED"
	EventCategoryPDBRestored                 EventCategory = "PDB_RESTORED"
	EventCategoryWebhookMutation             EventCategory = "WEBHOOK_MUTATION"
	EventCategoryPODEviction                 EventCategory = "POD_EVICTION"
	EventCategoryOOMEvent                    EventCategory = "OOM_EVENT"
	EventCategoryNodeOverloadTaintAdded      EventCategory = "NODE_OVERLOAD_TAINT_ADDED"
	EventCategoryNodeOverloadTaintRemoved    EventCategory = "NODE_OVERLOAD_TAINT_REMOVED"
)

// AuditPayload holds message, target, and details for an audit event.
type AuditPayload struct {
	Message string                 `json:"message,omitempty"`
	Target  interface{}            `json:"target,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// AuditEvent represents a single audit record.
type AuditEvent struct {
	ClusterID string        `json:"cluster_id"`
	Type      EventType     `json:"type"`
	Category  EventCategory `json:"category"`
	Payload   AuditPayload  `json:"payload"`
}

// AuditEventRecord is an audit event with its database timestamp (for API responses).
type AuditEventRecord struct {
	AuditEvent
	CreatedAt time.Time `json:"created_at"`
}
