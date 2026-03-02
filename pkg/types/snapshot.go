package types

import "time"

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

// SnapshotRecord is a snapshot with its database timestamp (for API responses).
type SnapshotRecord struct {
	SnapshotPayload
	CreatedAt time.Time `json:"created_at"`
}

type HistoricalTimelineMetric string

const (
	HistoricalTimelineMetricCPU    HistoricalTimelineMetric = "cpu"
	HistoricalTimelineMetricMemory HistoricalTimelineMetric = "memory"
	HistoricalTimelineMetricCost   HistoricalTimelineMetric = "cost"
)

type HistoricalTimelineThreshold struct {
	Value float64 `json:"value"`
	Color string  `json:"color"`
}

type HistoricalTimelinePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type HistoricalTimelineItem struct {
	Legend    string                      `json:"legend"`
	Color     string                      `json:"color"`
	Threshold HistoricalTimelineThreshold `json:"threshold"`
	Data      HistoricalTimelinePoint     `json:"data"`
}

type HistoricalTimelineResponse struct {
	Data []HistoricalTimelineItem `json:"data"`
}
