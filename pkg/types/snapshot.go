package types

// SnapshotResourceMetrics holds the metrics for CPU or Memory in a snapshot.
// Current = cluster totals; WorkloadRequested = user's original manifest request; RecommendedRequested = our recommendation.
type SnapshotResourceMetrics struct {
	CurrentAllocatable   float64 `json:"currentAllocatable"`   // total allocatable
	CurrentRequested     float64 `json:"currentRequested"`     // total requested
	CurrentUtilized      float64 `json:"currentUtilized"`      // total utilized (from usage stats)
	WorkloadRequested    float64 `json:"workloadRequested"`    // total CPU/memory user set in original manifest
	RecommendedRequested float64 `json:"recommendedRequested"` // total we recommend should be requested
}

// SnapshotNodes holds node counts by health in the snapshot.
type SnapshotNodes struct {
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
}

// SnapshotPodsCount holds pod counts by status (e.g. "Running", "Pending").
type SnapshotPodsCount map[string]int

// SnapshotData is the JSON stored in the snapshots.data column.
type SnapshotData struct {
	CPU       SnapshotResourceMetrics `json:"cpu"`
	Memory    SnapshotResourceMetrics `json:"memory"`
	Nodes     SnapshotNodes           `json:"nodes"`
	PodsCount SnapshotPodsCount       `json:"podsCount"`
}

// SnapshotPayload is the in-memory payload for a single snapshot row (cluster-level, one per run).
type SnapshotPayload struct {
	ClusterID string       `json:"cluster_id"`
	Data      SnapshotData `json:"data"`
}
