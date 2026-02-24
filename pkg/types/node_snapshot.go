package types

import "time"

// NodeSnapshotResourceMetrics holds the metrics for CPU or Memory in a node snapshot.
// Current = cluster totals; WorkloadRequested = user's original manifest request; RecommendedRequested = our recommendation.
type NodeSnapshotResourceMetrics struct {
	CurrentAllocatable   float64 `json:"current_allocatable"`   // total allocatable
	CurrentRequested     float64 `json:"current_requested"`     // total requested
	CurrentUtilized      float64 `json:"current_utilized"`      // total utilized (from usage stats)
	WorkloadRequested    float64 `json:"workload_requested"`    // total CPU/memory user set in original manifest
	RecommendedRequested float64 `json:"recommended_requested"` // total we recommend should be requested
}

// NodeSnapshotPayload is the in-memory payload for a single snapshot row (cluster-level, one per run).
type NodeSnapshotPayload struct {
	ClusterID       string                      `json:"cluster_id"`
	Timestamp       time.Time                   `json:"timestamp"`
	CPU             NodeSnapshotResourceMetrics `json:"cpu"`
	Memory          NodeSnapshotResourceMetrics `json:"memory"`
	NodeCount       int                         `json:"node_count"`        // number of healthy (in-scope) nodes in the snapshot
	RunningPodCount int                         `json:"running_pod_count"` // number of running pods in the snapshot
	MetaData        string                      `json:"metadata"`          // optional JSON; empty means no metadata
}
