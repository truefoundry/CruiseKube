package types

import "time"

// NodeSnapshotResourceMetrics holds the four metrics for CPU or Memory in a node snapshot.
// Current = cluster totals; WorkloadRequested = user's original manifest request; RecommendedRequested = our recommendation.
type NodeSnapshotResourceMetrics struct {
	CurrentAllocatable   float64 `json:"current_allocatable"`   // total allocatable
	CurrentRequested     float64 `json:"current_requested"`     // total requested
	WorkloadRequested    float64 `json:"workload_requested"`    // total CPU/memory user set in original manifest
	RecommendedRequested float64 `json:"recommended_requested"` // total we recommend should be requested
}

// NodeSnapshotPayload is the in-memory payload for a single snapshot row (cluster-level, one per run).
type NodeSnapshotPayload struct {
	ClusterID string
	Timestamp time.Time
	CPU       NodeSnapshotResourceMetrics
	Memory    NodeSnapshotResourceMetrics
	MetaData  string // optional JSON; empty means no metadata
}
