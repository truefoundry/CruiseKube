package database

import (
	"time"
)

type RowStats struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClusterID   string    `json:"cluster_id" gorm:"column:clusterId;not null;index:idx_cluster_workload,unique;index:idx_cluster_updated;index:idx_cluster_generated"`
	WorkloadID  string    `json:"workload_id" gorm:"column:workloadId;not null;index:idx_cluster_workload,unique"`
	Stats       string    `json:"stats" gorm:"not null"`
	GeneratedAt time.Time `json:"generated_at" gorm:"column:generatedAt;not null;index:idx_cluster_generated"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:createdAt;autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updatedAt;autoUpdateTime;index:idx_cluster_updated"`
	Overrides   string    `json:"overrides" gorm:"default:'{}'"`
}

func (RowStats) TableName() string {
	return "stats"
}
