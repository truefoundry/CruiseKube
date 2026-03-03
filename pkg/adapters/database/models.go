package database

import (
	"time"
)

type Cluster struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID string    `gorm:"column:cluster_id;uniqueIndex"`
	Settings  string    `gorm:"column:settings;type:text;default:'{}'"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Cluster) TableName() string {
	return "clusters"
}

// Workload is the DB row for a workload (stats payload + overrides) per cluster.
type Workload struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID   string    `gorm:"column:cluster_id;index;index:idx_workloads_cluster_updated,priority:1;index:idx_workloads_cluster_generated,priority:1;uniqueIndex:idx_workloads_cluster_workload"`
	WorkloadID  string    `gorm:"column:workload_id;index;uniqueIndex:idx_workloads_cluster_workload"`
	Stats       string    `gorm:"column:stats"` // JSON payload of workload stats
	GeneratedAt time.Time `gorm:"column:generated_at;index;index:idx_workloads_cluster_generated,priority:2"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime;index;index:idx_workloads_cluster_updated,priority:2"`
	Overrides   string    `gorm:"column:overrides;default:'{}'"`
}

func (Workload) TableName() string {
	return "workloads"
}

type OOMEvent struct {
	ID                 uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID          string    `gorm:"column:cluster_id;index;index:idx_oom_cluster_timestamp,priority:1;index:idx_oom_cluster_container_pod,priority:1;uniqueIndex:idx_oom_unique"`
	ContainerID        string    `gorm:"column:container_id;index;index:idx_oom_cluster_container_pod,priority:2;uniqueIndex:idx_oom_unique"`
	PodName            string    `gorm:"column:pod_name;index;index:idx_oom_cluster_container_pod,priority:3"`
	NodeName           string    `gorm:"column:node_name;index"`
	Namespace          string    `gorm:"column:namespace;index"`
	Timestamp          time.Time `gorm:"column:timestamp;index;index:idx_oom_cluster_timestamp,priority:2;uniqueIndex:idx_oom_unique"`
	MemoryLimit        int64     `gorm:"column:memory_limit;"`
	MemoryRequest      int64     `gorm:"column:memory_request;"`
	LastObservedMemory int64     `gorm:"column:last_observed_memory;"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime;index"`
}

func (OOMEvent) TableName() string {
	return "oom_events"
}

type PodResourceRecommendation struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID      string    `gorm:"column:cluster_id;index;index:idx_pod_rec_cluster_workload,priority:1"`
	WorkloadID     string    `gorm:"column:workload_id;index:idx_pod_rec_cluster_workload,priority:2"`
	NodeName       string    `gorm:"column:node_name"`
	Namespace      string    `gorm:"column:namespace"`
	Pod            string    `gorm:"column:pod"`
	Container      string    `gorm:"column:container"`
	Recommendation string    `gorm:"column:recommendation"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (PodResourceRecommendation) TableName() string {
	return "pod_resource_recommendations"
}

// AuditEventRow is the DB row for an audit event. Payload is JSON: { "message", "target", "before", "after", "details" }.
type AuditEventRow struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID string    `gorm:"column:cluster_id;index;index:idx_audit_events_cluster_created,priority:1;not null"`
	Cluster   Cluster   `gorm:"foreignKey:ClusterID;references:ClusterID"`
	Type      string    `gorm:"column:type;not null"`
	Category  string    `gorm:"column:category;index;not null"`
	Payload   string    `gorm:"column:payload;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index:idx_audit_events_cluster_created,priority:2"`
}

func (AuditEventRow) TableName() string {
	return "audit_events"
}

// Snapshot is the DB row for a cluster-level node stats snapshot (one per apply-recommendation run).
type Snapshot struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID string    `gorm:"column:cluster_id;index;index:idx_snapshots_cluster_created,priority:1;not null"`
	Cluster   Cluster   `gorm:"foreignKey:ClusterID;references:ClusterID"`
	Data      string    `gorm:"column:data;not null"` // JSON of SnapshotData (CPU, Memory, Nodes, PodsCount)
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index:idx_snapshots_cluster_created,priority:2"`
}

func (Snapshot) TableName() string {
	return "snapshots"
}
