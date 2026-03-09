package ports

import (
	"errors"
	"time"

	"github.com/truefoundry/cruisekube/pkg/types"
)

var (
	ErrWorkloadNotFound = errors.New("workload not found")
	ErrOOMEventNotFound = errors.New("OOM event not found")
	ErrSettingsNotFound = errors.New("cluster settings not found")
)

type Database interface {
	Close() error
	// Upsert
	UpsertStat(clusterID, workloadID string, stat types.WorkloadStat, generatedAt time.Time) error

	// Has
	HasRecentStat(clusterID, workloadID string, withinMinutes int) (bool, error)
	HasCluster(clusterID string) (bool, error)
	HasWorkloadForCluster(clusterID, workloadID string) (bool, error)

	// Get
	GetStatsForCluster(clusterID string) ([]types.WorkloadStat, error)
	GetWorkloadsInCluster(clusterID string) ([]*types.WorkloadInCluster, error)
	GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error)
	GetStatCountForCluster(clusterID string) (int, error)
	GetStatOverridesForWorkload(clusterID, workloadID string) (*types.Overrides, error)

	// Delete
	DeleteWorkloadsForCluster(clusterID string) error
	DeleteWorkload(clusterID, workloadID string) error
	DeleteWorkloadsNotInCluster(clusterID string, keepIDs []string) (int, error)

	// Update
	UpdateStatOverridesForWorkload(clusterID, workloadID string, overrides *types.Overrides) error

	// OOM Events
	InsertOOMEvent(event *types.OOMEvent) error
	GetOOMEventsByWorkload(clusterID, workloadID string, since time.Time) ([]types.OOMEvent, error)
	GetLatestOOMEventForContainer(clusterID, containerID, podName string) (*types.OOMEvent, error)
	DeleteOldOOMEvents(clusterID string, olderThan time.Time) (int64, error)

	// Pod Recommendations
	SavePodRecommendations(clusterID string, rows []types.PodResourceRecommendationRow) error
	GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error)
	GetPodRecommendationsForWorkload(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error)

	// Audit
	InsertAuditEvent(clusterID string, event types.AuditEvent) error
	GetAuditEvents(clusterID string, since time.Time) ([]types.AuditEventRecord, error)
	GetAuditEventsForWorkload(clusterID, workloadID string, since time.Time) ([]types.AuditEventRecord, error)
	// Node Snapshots
	InsertSnapshot(snapshot *types.SnapshotPayload) error
	GetSnapshotsInRange(clusterID string, startTime, endTime time.Time) ([]types.SnapshotRecord, error)
	// Settings
	GetClusterSettings(clusterID string) (*types.ClusterSettings, error)
	UpdateClusterSettings(clusterID string, settings *types.ClusterSettings) error
}
