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
	EventCategoryResourceApplied       EventCategory = "RESOURCE_APPLIED"
	EventCategoryDNDAnnotationRestored EventCategory = "DND_ANNOTATION_RESTORED"
	EventCategoryDNDAnnotationRemoved  EventCategory = "DND_ANNOTATION_REMOVED"
	EventCategoryWebhookMutation       EventCategory = "WEBHOOK_MUTATION"
	EventCategoryEviction              EventCategory = "EVICTION"
	EventCategoryConfigChange          EventCategory = "CONFIG_CHANGE"
)

// AuditMetaData is key-value metadata for an audit event (no fixed fields; use for arbitrary extras).
type AuditMetaData map[string]string

// AuditPayload holds message, target, and optional before/after state for an audit event.
// Target can be a string (e.g. "namespace/name"), or an object e.g. {"kind":"Pod","name":"...","namespace":"..."}, or other shape.
// Before and After describe the state change as key-value pairs.
type AuditPayload struct {
	Message string                 `json:"message,omitempty"`
	Target  interface{}            `json:"target,omitempty"`
	Before  map[string]interface{} `json:"before,omitempty"`
	After   map[string]interface{} `json:"after,omitempty"`
}

// AuditEvent represents a single audit record per the CruiseKube Audit spec.
// Timestamp is in GMT (UTC). Payload holds message, target (string or object), and optional before/after. MetaData holds only extras.
type AuditEvent struct {
	Timestamp time.Time     `json:"timestamp"`
	ClusterID string        `json:"cluster_id"`
	Type      EventType     `json:"type"`
	Category  EventCategory `json:"category"`
	Source    string        `json:"source"`
	Payload   AuditPayload  `json:"payload"`
	MetaData  AuditMetaData `json:"metadata,omitempty"`
}
