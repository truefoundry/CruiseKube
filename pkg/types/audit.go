package types

import (
	"encoding/json"
	"time"
)

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
	EventCategoryRecommendationGenerated EventCategory = "RECOMMENDATION_GENERATED"
	EventCategoryRecommendationSkipped   EventCategory = "RECOMMENDATION_SKIPPED"
	EventCategoryResourceApplied         EventCategory = "RESOURCE_APPLIED"
	EventCategoryResourceReverted       EventCategory = "RESOURCE_REVERTED"
	EventCategoryWebhookMutation         EventCategory = "WEBHOOK_MUTATION"
	EventCategoryDisruptionOverride      EventCategory = "DISRUPTION_OVERRIDE"
	EventCategoryEviction                EventCategory = "EVICTION"
	EventCategoryNodeImpact               EventCategory = "NODE_IMPACT"
	EventCategoryCostImpact              EventCategory = "COST_IMPACT"
	EventCategoryStatsCollected          EventCategory = "STATS_COLLECTED"
	EventCategoryConfigChange            EventCategory = "CONFIG_CHANGE"
	EventCategorySystemEvent             EventCategory = "SYSTEM_EVENT"
)

// AuditMetaData holds structured metadata for an audit event (Namespace, Kind, Name + extensible).
type AuditMetaData struct {
	Namespace string            `json:"namespace,omitempty"`
	Kind      string            `json:"kind,omitempty"`
	Name      string            `json:"name,omitempty"`
	Extras    map[string]string `json:"extras,omitempty"`
}

// AuditEvent represents a single audit record per the CruiseKube Audit spec.
// Timestamp is in GMT (UTC). Automated is true when created by the system, false when created by user action.
type AuditEvent struct {
	Timestamp time.Time      `json:"timestamp"`
	ClusterID string         `json:"cluster_id"`
	Type      EventType      `json:"type"`
	Automated bool           `json:"automated"`
	Category  EventCategory `json:"category"`
	Source    string         `json:"source"`
	Target    string         `json:"target,omitempty"`
	Message   string         `json:"message"`
	MetaData  AuditMetaData  `json:"metadata,omitempty"`
}

// ToJSON returns the metadata extras as JSON for DB storage; returns empty string if Extras is nil/empty.
func (m AuditMetaData) ExtrasJSON() string {
	if len(m.Extras) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(m.Extras)
	return string(b)
}
