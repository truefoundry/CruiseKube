package sqlite

import (
	"time"
)

type RowStats struct {
	ID          int       `json:"id"`
	ClusterID   string    `json:"cluster_id"`
	WorkloadID  string    `json:"workload_id"`
	Stats       string    `json:"stats"`
	GeneratedAt time.Time `json:"generated_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Overrides   string    `json:"overrides"`
}
