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

// EventCategory is the category/name of the audit event per the CruiseKube Audit spec.
type EventCategory string

const (
	EventCategoryResourceUpdated        EventCategory = "RESOURCE_UPDATED"
	EventCategoryPODDisruptionUnblocked EventCategory = "POD_DISRUPTION_UNBLOCKED"
	EventCategoryPODDisruptionRestored  EventCategory = "POD_DISRUPTION_RESTORED"
	EventCategoryWebhookMutation        EventCategory = "WEBHOOK_MUTATION"
	EventCategoryPODEviction            EventCategory = "POD_EVICTION"
)

// AuditPayload holds message, target, optional before/after state, and details for an audit event.
// Target can be a string or an object e.g. {"kind":"Pod","name":"...","namespace":"..."}.
// Before and After use consistent units: CPU in millicores (cpu_request_millis, cpu_limit_millis), memory in MB (memory_request_mb, memory_limit_mb).
// Details holds any other key-value information to track.
type AuditPayload struct {
	Message string                 `json:"message,omitempty"`
	Target  interface{}            `json:"target,omitempty"`
	Before  map[string]interface{} `json:"before,omitempty"`
	After   map[string]interface{} `json:"after,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// AuditEvent represents a single audit record. Timestamp is in GMT (UTC).
type AuditEvent struct {
	Timestamp time.Time     `json:"timestamp"`
	ClusterID string        `json:"cluster_id"`
	Type      EventType     `json:"type"`
	Category  EventCategory `json:"category"`
	Payload   AuditPayload  `json:"payload"`
}
