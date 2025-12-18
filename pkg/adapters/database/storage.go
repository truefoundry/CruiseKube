package database

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/truefoundry/cruisekube/pkg/types"
	"gorm.io/gorm"
)

// GormStorage implements the Storage interface using GORM
// This is the shared implementation that works with any GORM-supported database
type GormStorage struct {
	db *gorm.DB
}

// NewGormStorage creates a new GormStorage instance with the provided GORM DB client
func NewGormStorage(db *gorm.DB) (*GormStorage, error) {
	storage := &GormStorage{db: db}

	// Auto-migrate the RowStats model
	if err := storage.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return storage, nil
}

func (s *GormStorage) createTables() error {
	// Auto-migrate the RowStats model
	if err := s.db.AutoMigrate(&RowStats{}); err != nil {
		return fmt.Errorf("failed to auto-migrate RowStats: %w", err)
	}
	return nil
}

func (s *GormStorage) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *GormStorage) UpsertStat(clusterID, workloadID string, stat types.WorkloadStat, generatedAt time.Time) error {
	statsJSON, err := json.Marshal(stat)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	rowStat := RowStats{
		ClusterID:   clusterID,
		WorkloadID:  workloadID,
		Stats:       string(statsJSON),
		GeneratedAt: generatedAt,
	}

	// Use GORM's Clauses for upsert functionality
	result := s.db.Where("clusterId = ? AND workloadId = ?", clusterID, workloadID).
		Assign(RowStats{
			Stats:       string(statsJSON),
			GeneratedAt: generatedAt,
		}).
		FirstOrCreate(&rowStat)

	if result.Error != nil {
		return fmt.Errorf("failed to upsert stats: %w", result.Error)
	}

	return nil
}

func (s *GormStorage) HasRecentStat(clusterID, workloadID string, withinMinutes int) (bool, error) {
	cutoffTime := time.Now().Add(-time.Duration(withinMinutes) * time.Minute)

	var count int64
	err := s.db.Model(&RowStats{}).
		Where("clusterId = ? AND workloadId = ? AND generatedAt > ?", clusterID, workloadID, cutoffTime).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check recent stats: %w", err)
	}

	return count > 0, nil
}

func (s *GormStorage) HasStatForCluster(clusterID string) (bool, error) {
	count, err := s.GetStatCountForCluster(clusterID)
	return err == nil && count > 0, nil
}

func (s *GormStorage) HasStatForWorkload(clusterID, workloadID string) (bool, error) {
	count, err := s.GetStatCountForWorkload(clusterID, workloadID)
	return err == nil && count > 0, nil
}

func (s *GormStorage) GetStatsForCluster(clusterID string) ([]types.WorkloadStat, error) {
	var rowStats []RowStats
	err := s.db.Where("clusterId = ?", clusterID).
		Order("updatedAt DESC").
		Find(&rowStats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query cluster stats: %w", err)
	}

	var stats []types.WorkloadStat
	for _, row := range rowStats {
		var stat types.WorkloadStat
		if err := json.Unmarshal([]byte(row.Stats), &stat); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
		}

		stat.UpdatedAt = row.UpdatedAt
		stats = append(stats, stat)
	}

	return stats, nil
}

func (s *GormStorage) GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error) {
	var rowStat RowStats
	err := s.db.Where("clusterId = ? AND workloadId = ?", clusterID, workloadID).
		First(&rowStat).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("workload stat not found for cluster %s, workload %s", clusterID, workloadID)
		}
		return nil, fmt.Errorf("failed to query workload stat: %w", err)
	}

	var stat types.WorkloadStat
	if err := json.Unmarshal([]byte(rowStat.Stats), &stat); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	stat.UpdatedAt = rowStat.UpdatedAt
	return &stat, nil
}

func (s *GormStorage) GetStatCountForCluster(clusterID string) (int, error) {
	var count int64
	err := s.db.Model(&RowStats{}).
		Where("clusterId = ?", clusterID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count stats: %w", err)
	}

	return int(count), nil
}

func (s *GormStorage) GetStatCountForWorkload(clusterID, workloadID string) (int, error) {
	var count int64
	err := s.db.Model(&RowStats{}).
		Where("clusterId = ? AND workloadId = ?", clusterID, workloadID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count stats: %w", err)
	}

	return int(count), nil
}

func (s *GormStorage) GetStatOverridesForWorkload(clusterID, workloadID string) (*types.Overrides, error) {
	var rowStat RowStats
	err := s.db.Select("overrides").
		Where("clusterId = ? AND workloadId = ?", clusterID, workloadID).
		First(&rowStat).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("workload overrides not found for cluster %s, workload %s", clusterID, workloadID)
		}
		return nil, fmt.Errorf("failed to query workload overrides: %w", err)
	}

	var overrides types.Overrides
	if err := json.Unmarshal([]byte(rowStat.Overrides), &overrides); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overrides: %w", err)
	}

	return &overrides, nil
}

func (s *GormStorage) DeleteStatsForCluster(clusterID string) error {
	err := s.db.Where("clusterId = ?", clusterID).Delete(&RowStats{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete cluster stats: %w", err)
	}

	return nil
}

func (s *GormStorage) DeleteStatForWorkload(clusterID, workloadID string) error {
	result := s.db.Where("clusterId = ? AND workloadId = ?", clusterID, workloadID).Delete(&RowStats{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete workload stat: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("workload stat not found")
	}

	return nil
}

func (s *GormStorage) UpdateStatOverridesForWorkload(clusterID, workloadID string, overrides *types.Overrides) error {
	overridesJSON, err := json.Marshal(overrides)
	if err != nil {
		return fmt.Errorf("failed to marshal overrides: %w", err)
	}

	result := s.db.Model(&RowStats{}).
		Where("clusterId = ? AND workloadId = ?", clusterID, workloadID).
		Update("overrides", string(overridesJSON))

	if result.Error != nil {
		return fmt.Errorf("failed to update workload overrides: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("workload not found: cluster %s, workload %s", clusterID, workloadID)
	}

	return nil
}
