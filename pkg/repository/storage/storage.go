package storage

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/types"
)

var (
	ErrWorkloadNotFound = ports.ErrWorkloadNotFound
	ErrOOMEventNotFound = ports.ErrOOMEventNotFound
	ErrSettingsNotFound = ports.ErrSettingsNotFound
)

const BytesPerMB = 1_000_000

var Stg *Storage

type Storage struct {
	DB ports.Database
}

func NewStorageRepo(db ports.Database) (*Storage, error) {
	return &Storage{DB: db}, nil
}

func (s *Storage) WriteClusterStats(clusterID string, statsResponse types.StatsResponse, generatedAt time.Time) error {
	for _, stat := range statsResponse.Stats {
		workloadID := strings.ReplaceAll(stat.WorkloadIdentifier, "/", ":")
		if err := s.DB.UpsertStat(clusterID, workloadID, stat, generatedAt); err != nil {
			return fmt.Errorf("failed to write workload stat %s: %w", workloadID, err)
		}
	}

	return nil
}

func (s *Storage) ReadClusterStats(clusterID string, target *types.StatsResponse) error {
	stats, err := s.DB.GetStatsForCluster(clusterID)
	if err != nil {
		return fmt.Errorf("failed to read cluster stats: %w", err)
	}

	target.Stats = stats
	return nil
}

// GetStatForWorkload returns a single workload stat for the cluster and workload.
// workloadID is the colon-separated identifier (e.g. Deployment:namespace:name).
func (s *Storage) GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error) {
	stat, err := s.DB.GetStatForWorkload(clusterID, workloadID)
	if errors.Is(err, ports.ErrWorkloadNotFound) {
		return nil, fmt.Errorf("workload %s not found in cluster %s: %w", workloadID, clusterID, ErrWorkloadNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get stat for workload %s in cluster %s: %w", workloadID, clusterID, err)
	}
	return stat, nil
}

func (s *Storage) ClusterStatsExists(clusterID string) (bool, error) {
	exists, err := s.DB.HasCluster(clusterID)
	if err != nil {
		return false, fmt.Errorf("failed to check cluster stats existence: %w", err)
	}
	return exists, nil
}

func (s *Storage) HasRecentStats(clusterID, workloadID string, withinMinutes int) (bool, error) {
	hasRecent, err := s.DB.HasRecentStat(clusterID, workloadID, withinMinutes)
	if err != nil {
		return false, fmt.Errorf("failed to check recent stats: %w", err)
	}
	return hasRecent, nil
}

func (s *Storage) UpdateWorkloadOverrides(clusterID, workloadID string, overrides *types.Overrides) error {
	exists, err := s.DB.HasWorkloadForCluster(clusterID, workloadID)
	if err != nil {
		return fmt.Errorf("failed to get stats record: %w", err)
	}
	if !exists {
		return fmt.Errorf("workload %s not found in cluster %s: %w", workloadID, clusterID, ErrWorkloadNotFound)
	}
	if err := s.DB.UpdateStatOverridesForWorkload(clusterID, workloadID, overrides); err != nil {
		return fmt.Errorf("failed to update workload overrides: %w", err)
	}
	return nil
}

func (s *Storage) BatchUpdateWorkloadOverrides(clusterID string, workloadIDs []string, overrides *types.Overrides) ([]string, []string, error) {
	updatedIDs, err := s.DB.BatchUpdateStatOverridesForWorkloads(clusterID, workloadIDs, overrides)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to batch update workload overrides: %w", err)
	}

	updatedSet := make(map[string]struct{}, len(updatedIDs))
	for _, id := range updatedIDs {
		updatedSet[id] = struct{}{}
	}

	notFound := make([]string, 0)
	for _, id := range workloadIDs {
		if _, ok := updatedSet[id]; !ok {
			notFound = append(notFound, id)
		}
	}

	return updatedIDs, notFound, nil
}

func (s *Storage) GetWorkloadOverrides(clusterID, workloadID string) (*types.Overrides, error) {
	overrides, err := s.DB.GetStatOverridesForWorkload(clusterID, workloadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload overrides: %w", err)
	}
	return overrides, nil
}

func (s *Storage) GetAllStatsForCluster(clusterID string) ([]types.WorkloadStat, error) {
	stats, err := s.DB.GetStatsForCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats for cluster: %w", err)
	}
	return stats, nil
}

// GetWorkloadsInCluster returns all workloads for a cluster (stat + overrides) in a single DB call.
// Use methods on each WorkloadInCluster to get Stat, Overrides, or OverridesWithDefaults().
func (s *Storage) GetWorkloadsInCluster(clusterID string) ([]*types.WorkloadInCluster, error) {
	workloads, err := s.DB.GetWorkloadsInCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workloads for cluster: %w", err)
	}
	return workloads, nil
}

// DeleteStaleWorkloads removes workloads from the DB that are no longer present in the cluster.
// currentWorkloadIds is the complete set of workload keys currently observed in the cluster.
func (s *Storage) DeleteStaleWorkloads(clusterID string, currentWorkloadIds map[string]struct{}) (int, error) {
	keepIDs := make([]string, 0, len(currentWorkloadIds))
	for id := range currentWorkloadIds {
		keepIDs = append(keepIDs, id)
	}
	deleted, err := s.DB.DeleteWorkloadsNotInCluster(clusterID, keepIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to delete stale workloads: %w", err)
	}
	return deleted, nil
}

// OOM Event Methods
func (s *Storage) InsertOOMEvent(event *types.OOMEvent) error {
	if err := s.DB.InsertOOMEvent(event); err != nil {
		return fmt.Errorf("failed to insert OOM event: %w", err)
	}
	return nil
}

func (s *Storage) GetOOMEventsByWorkload(clusterID, workloadID string, since time.Time) ([]types.OOMEvent, error) {
	events, err := s.DB.GetOOMEventsByWorkload(clusterID, workloadID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get OOM events for workload: %w", err)
	}
	return events, nil
}

func (s *Storage) GetLatestOOMEventForContainer(clusterID, containerID, podName string) (*types.OOMEvent, error) {
	event, err := s.DB.GetLatestOOMEventForContainer(clusterID, containerID, podName)
	if errors.Is(err, ports.ErrOOMEventNotFound) {
		return nil, fmt.Errorf("no OOM event for container %s in pod %s: %w", containerID, podName, ErrOOMEventNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get latest OOM event for container %s in pod %s: %w", containerID, podName, err)
	}
	return event, nil
}

func (s *Storage) UpdateOOMMemoryForContainer(clusterID, workloadID, containerName string, oomMemoryBytes int64) error {
	stat, err := s.GetStatForWorkload(clusterID, workloadID)
	if err != nil {
		return fmt.Errorf("failed to get stat for workload: %w", err)
	}

	containerFound := false
	for i := range stat.ContainerStats {
		if stat.ContainerStats[i].ContainerName != containerName {
			continue
		}
		if stat.ContainerStats[i].MemoryStats == nil {
			stat.ContainerStats[i].MemoryStats = &types.MemoryStats{}
		}
		stat.ContainerStats[i].MemoryStats.OOMMemory = float64(oomMemoryBytes) / BytesPerMB

		containerFound = true
		break
	}

	if !containerFound {
		return fmt.Errorf("container %s not found in workload %s stats", containerName, workloadID)
	}

	if err := s.DB.UpsertStat(clusterID, workloadID, *stat, time.Now()); err != nil {
		return fmt.Errorf("failed to update stat with OOM memory: %w", err)
	}

	return nil
}

func (s *Storage) DeleteOldOOMEvents(clusterID string, retentionDays int) (int64, error) {
	cutoffTime := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	rowsAffected, err := s.DB.DeleteOldOOMEvents(clusterID, cutoffTime)
	if err != nil {
		return rowsAffected, fmt.Errorf("failed to delete old OOM events: %w", err)
	}
	return rowsAffected, nil
}

func (s *Storage) SavePodRecommendations(clusterID string, rows []types.PodResourceRecommendationRow) error {
	if err := s.DB.SavePodRecommendations(clusterID, rows); err != nil {
		return fmt.Errorf("failed to save pod recommendations: %w", err)
	}
	return nil
}

func (s *Storage) GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error) {
	rows, err := s.DB.GetPodRecommendationsForCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod recommendations for cluster: %w", err)
	}
	return rows, nil
}

func (s *Storage) GetPodRecommendationsForWorkload(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error) {
	rows, err := s.DB.GetPodRecommendationsForWorkload(clusterID, workloadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod recommendations for workload: %w", err)
	}
	return rows, nil
}

func (s *Storage) InsertSnapshot(snapshot *types.SnapshotPayload) error {
	if err := s.DB.InsertSnapshot(snapshot); err != nil {
		return fmt.Errorf("failed to insert node snapshot: %w", err)
	}
	return nil
}

// GetSnapshotsInRange returns snapshots for the cluster in [startTime, endTime].
func (s *Storage) GetSnapshotsInRange(clusterID string, startTime, endTime time.Time) ([]types.SnapshotRecord, error) {
	snapshots, err := s.DB.GetSnapshotsInRange(clusterID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots for cluster %s in range: %w", clusterID, err)
	}
	return snapshots, nil
}

func (s *Storage) GetSettings(clusterID string) (*types.ClusterSettings, error) {
	settings, err := s.DB.GetClusterSettings(clusterID)
	if errors.Is(err, ports.ErrSettingsNotFound) {
		return nil, fmt.Errorf("no settings for cluster %s: %w", clusterID, ErrSettingsNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get settings for cluster %s: %w", clusterID, err)
	}
	return settings, nil
}

func (s *Storage) UpdateSettings(clusterID string, settings *types.ClusterSettings) error {
	if err := s.DB.UpdateClusterSettings(clusterID, settings); err != nil {
		return fmt.Errorf("failed to upsert settings: %w", err)
	}
	return nil
}

// GetAuditEvents returns audit events for the cluster since the given time.
func (s *Storage) GetAuditEvents(clusterID string, since time.Time) ([]types.AuditEventRecord, error) {
	events, err := s.DB.GetAuditEvents(clusterID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit events for cluster %s: %w", clusterID, err)
	}
	return events, nil
}

// GetAuditEventsForWorkload returns audit events for the given workload in the cluster since the given time.
func (s *Storage) GetAuditEventsForWorkload(clusterID, workloadID string, since time.Time) ([]types.AuditEventRecord, error) {
	events, err := s.DB.GetAuditEventsForWorkload(clusterID, workloadID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit events for workload %s: %w", workloadID, err)
	}
	return events, nil
}
