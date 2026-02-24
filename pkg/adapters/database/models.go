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
	ClusterID   string    `gorm:"column:cluster_id;index;uniqueIndex:idx_workloads_cluster_workload"`
	WorkloadID  string    `gorm:"column:workload_id;index;uniqueIndex:idx_workloads_cluster_workload"`
	Stats       string    `gorm:"column:stats"` // JSON payload of workload stats
	GeneratedAt time.Time `gorm:"column:generated_at;index"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime;index"`
	Overrides   string    `gorm:"column:overrides;default:'{}'"`
}

func (Workload) TableName() string {
	return "workloads"
}

type OOMEvent struct {
	ID                 uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID          string    `gorm:"column:cluster_id;index;uniqueIndex:idx_oom_unique"`
	ContainerID        string    `gorm:"column:container_id;index;uniqueIndex:idx_oom_unique"`
	PodName            string    `gorm:"column:pod_name;index"`
	NodeName           string    `gorm:"column:node_name;index"`
	Namespace          string    `gorm:"column:namespace;index"`
	Timestamp          time.Time `gorm:"column:timestamp;index;uniqueIndex:idx_oom_unique"`
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
	ClusterID      string    `gorm:"column:cluster_id"`
	WorkloadID     string    `gorm:"column:workload_id"`
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
