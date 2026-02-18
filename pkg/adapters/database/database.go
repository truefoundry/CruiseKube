package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/truefoundry/cruisekube/pkg/adapters/database/clients"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DatabaseConfig holds configuration for database connections
type DatabaseConfig struct {
	Type     string `yaml:"type" json:"type"`         // "sqlite" or "postgres"
	Host     string `yaml:"host" json:"host"`         // For postgres
	Port     int    `yaml:"port" json:"port"`         // For postgres
	Database string `yaml:"database" json:"database"` // Database name or file path
	Username string `yaml:"username" json:"username"` // For postgres
	Password string `yaml:"password" json:"password"` // For postgres
	SSLMode  string `yaml:"sslmode" json:"sslmode"`   // For postgres
}

// NewDatabase creates a new storage instance based on the configuration
func NewDatabase(config DatabaseConfig) (ports.Database, error) {
	// Create the appropriate client factory
	clientFactory, err := clients.CreateClientFactory(clients.FactoryConfig{
		Type:     config.Type,
		Host:     config.Host,
		Port:     config.Port,
		Database: config.Database,
		Username: config.Username,
		Password: config.Password,
		SSLMode:  config.SSLMode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client factory: %w", err)
	}

	// Create the database client
	db, err := clientFactory.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create database client: %w", err)
	}

	// Create the shared storage implementation
	return NewGormDB(db)
}

// GormDB implements the Storage interface using GORM
// This is the shared implementation that works with any GORM-supported database
type GormDB struct {
	db *gorm.DB
}

// NewGormDB creates a new GormStorage instance with the provided GORM DB client
func NewGormDB(db *gorm.DB) (*GormDB, error) {
	gormDB := &GormDB{db: db}

	if err := gormDB.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return gormDB, nil
}

func (s *GormDB) createTables() error {
	// One-time migration: rename legacy "stats" table to "workloads"
	if s.db.Migrator().HasTable("stats") && !s.db.Migrator().HasTable("workloads") {
		if err := s.db.Exec("ALTER TABLE stats RENAME TO workloads").Error; err != nil {
			return fmt.Errorf("failed to rename stats table to workloads: %w", err)
		}
	}
	if err := s.db.AutoMigrate(&Workload{}); err != nil {
		return fmt.Errorf("failed to auto-migrate workloads: %w", err)
	}
	if err := s.db.AutoMigrate(&OOMEvent{}); err != nil {
		return fmt.Errorf("failed to auto-migrate OOMEvent: %w", err)
	}
	if err := s.db.AutoMigrate(&PodResourceRecommendation{}); err != nil {
		return fmt.Errorf("failed to auto-migrate PodResourceRecommendation: %w", err)
	}
	return nil
}

func (s *GormDB) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	return nil
}

func (s *GormDB) UpsertStat(clusterID, workloadID string, stat types.WorkloadStat, generatedAt time.Time) error {
	statsJSON, err := json.Marshal(stat)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	row := Workload{
		ClusterID:   clusterID,
		WorkloadID:  workloadID,
		Stats:       string(statsJSON),
		GeneratedAt: generatedAt,
	}

	// Use GORM's Clauses for upsert functionality
	result := s.db.Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).
		Assign(Workload{
			Stats:       string(statsJSON),
			GeneratedAt: generatedAt,
		}).
		FirstOrCreate(&row)

	if result.Error != nil {
		return fmt.Errorf("failed to upsert stats: %w", result.Error)
	}

	return nil
}

func (s *GormDB) HasRecentStat(clusterID, workloadID string, withinMinutes int) (bool, error) {
	cutoffTime := time.Now().Add(-time.Duration(withinMinutes) * time.Minute)

	var count int64
	err := s.db.Model(&Workload{}).
		Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).
		Where("generated_at > ?", cutoffTime).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check recent stats: %w", err)
	}

	return count > 0, nil
}

func (s *GormDB) HasCluster(clusterID string) (bool, error) {
	count, err := s.GetStatCountForCluster(clusterID)
	return err == nil && count > 0, nil
}

func (s *GormDB) HasWorkloadForCluster(clusterID, workloadID string) (bool, error) {
	count, err := s.getWorkloadCountForCluster(clusterID, workloadID)
	return err == nil && count > 0, nil
}

func (s *GormDB) GetStatsForCluster(clusterID string) ([]types.WorkloadStat, error) {
	var rows []Workload
	err := s.db.Where(&Workload{ClusterID: clusterID}).
		Order("updated_at DESC").
		Find(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query cluster stats: %w", err)
	}

	var stats []types.WorkloadStat
	for _, row := range rows {
		var stat types.WorkloadStat
		if err := json.Unmarshal([]byte(row.Stats), &stat); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
		}

		stat.UpdatedAt = row.UpdatedAt
		stats = append(stats, stat)
	}

	return stats, nil
}

func (s *GormDB) GetStatsForClusterUpdatedSince(clusterID string, since time.Time) ([]types.WorkloadStat, error) {
	var rows []Workload
	err := s.db.Where(&Workload{ClusterID: clusterID}).
		Where("updated_at > ?", since).
		Order("updated_at DESC").
		Find(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query cluster stats: %w", err)
	}

	var stats []types.WorkloadStat
	for _, row := range rows {
		var stat types.WorkloadStat
		if err := json.Unmarshal([]byte(row.Stats), &stat); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
		}

		stat.UpdatedAt = row.UpdatedAt
		stats = append(stats, stat)
	}

	return stats, nil
}

func (s *GormDB) GetWorkloadsInCluster(clusterID string, since time.Time) ([]*types.WorkloadInCluster, error) {
	var rows []Workload
	q := s.db.Where(&Workload{ClusterID: clusterID})
	if !since.IsZero() {
		q = q.Where("updated_at > ?", since)
	}
	err := q.Order("updated_at DESC").Find(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query workloads for cluster: %w", err)
	}

	out := make([]*types.WorkloadInCluster, 0, len(rows))
	for _, row := range rows {
		var stat types.WorkloadStat
		if err := json.Unmarshal([]byte(row.Stats), &stat); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stats for workload %s: %w", row.WorkloadID, err)
		}
		stat.UpdatedAt = row.UpdatedAt

		var overrides *types.Overrides
		if row.Overrides != "" && row.Overrides != "{}" {
			var o types.Overrides
			if err := json.Unmarshal([]byte(row.Overrides), &o); err != nil {
				return nil, fmt.Errorf("failed to unmarshal overrides for workload %s: %w", row.WorkloadID, err)
			}
			overrides = &o
		}

		out = append(out, &types.WorkloadInCluster{
			ClusterID:   clusterID,
			WorkloadID:  row.WorkloadID,
			Stat:        &stat,
			Overrides:   overrides,
			GeneratedAt: row.GeneratedAt,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}

	return out, nil
}

func (s *GormDB) GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error) {
	var row Workload
	err := s.db.Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).
		First(&row).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("workload stat not found for cluster %s, workload %s", clusterID, workloadID)
		}
		return nil, fmt.Errorf("failed to query workload stat: %w", err)
	}

	var stat types.WorkloadStat
	if err := json.Unmarshal([]byte(row.Stats), &stat); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	stat.UpdatedAt = row.UpdatedAt
	return &stat, nil
}

func (s *GormDB) GetStatCountForCluster(clusterID string) (int, error) {
	var count int64
	err := s.db.Model(&Workload{}).
		Where(&Workload{ClusterID: clusterID}).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count stats: %w", err)
	}

	return int(count), nil
}

func (s *GormDB) getWorkloadCountForCluster(clusterID, workloadID string) (int, error) {
	var count int64
	err := s.db.Model(&Workload{}).
		Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count stats: %w", err)
	}

	return int(count), nil
}

func (s *GormDB) GetStatOverridesForWorkload(clusterID, workloadID string) (*types.Overrides, error) {
	var row Workload
	err := s.db.Select("overrides").
		Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).
		First(&row).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("workload overrides not found for cluster %s, workload %s", clusterID, workloadID)
		}
		return nil, fmt.Errorf("failed to query workload overrides: %w", err)
	}

	var overrides types.Overrides
	if err := json.Unmarshal([]byte(row.Overrides), &overrides); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overrides: %w", err)
	}

	return &overrides, nil
}

func (s *GormDB) DeleteWorkloadsForCluster(clusterID string) error {
	err := s.db.Where(&Workload{ClusterID: clusterID}).Delete(&Workload{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete cluster stats: %w", err)
	}

	return nil
}

func (s *GormDB) DeleteWorkload(clusterID, workloadID string) error {
	result := s.db.Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).Delete(&Workload{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete workload stat: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("workload stat not found")
	}

	return nil
}

func (s *GormDB) UpdateStatOverridesForWorkload(clusterID, workloadID string, overrides *types.Overrides) error {
	overridesJSON, err := json.Marshal(overrides)
	if err != nil {
		return fmt.Errorf("failed to marshal overrides: %w", err)
	}

	result := s.db.Model(&Workload{}).
		Where(&Workload{ClusterID: clusterID, WorkloadID: workloadID}).
		Update("overrides", string(overridesJSON))

	if result.Error != nil {
		return fmt.Errorf("failed to update workload overrides: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("workload not found: cluster %s, workload %s", clusterID, workloadID)
	}

	return nil
}

func (s *GormDB) InsertOOMEvent(event *types.OOMEvent) error {
	dbEvent := OOMEvent{
		ClusterID:          event.ClusterID,
		ContainerID:        event.ContainerID,
		PodName:            event.PodName,
		NodeName:           event.NodeName,
		Namespace:          event.Namespace,
		Timestamp:          event.Timestamp,
		MemoryLimit:        event.MemoryLimit,
		MemoryRequest:      event.MemoryRequest,
		LastObservedMemory: event.LastObservedMemory,
	}

	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cluster_id"}, {Name: "container_id"}, {Name: "timestamp"}},
		DoNothing: true,
	}).Create(&dbEvent)

	if result.Error != nil {
		return fmt.Errorf("failed to insert OOM event: %w", result.Error)
	}

	return nil
}

func (s *GormDB) GetLatestOOMEventForContainer(clusterID, containerID, podName string) (*types.OOMEvent, error) {
	var dbEvent OOMEvent
	err := s.db.Where("cluster_id = ? AND container_id = ? AND pod_name = ?", clusterID, containerID, podName).
		Order("timestamp DESC").
		First(&dbEvent).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get OOM event for container %s: %w", containerID, err)
	}

	return &types.OOMEvent{
		ID:                 dbEvent.ID,
		ClusterID:          dbEvent.ClusterID,
		ContainerID:        dbEvent.ContainerID,
		PodName:            dbEvent.PodName,
		NodeName:           dbEvent.NodeName,
		Namespace:          dbEvent.Namespace,
		Timestamp:          dbEvent.Timestamp,
		MemoryLimit:        dbEvent.MemoryLimit,
		MemoryRequest:      dbEvent.MemoryRequest,
		LastObservedMemory: dbEvent.LastObservedMemory,
		CreatedAt:          dbEvent.CreatedAt,
		UpdatedAt:          dbEvent.UpdatedAt,
	}, nil
}

func (s *GormDB) GetOOMEventsByWorkload(clusterID, workloadID string, since time.Time) ([]types.OOMEvent, error) {
	var dbEvents []OOMEvent
	likePattern := workloadID + ":%"
	err := s.db.Where("cluster_id = ? AND container_id LIKE ? AND timestamp >= ?", clusterID, likePattern, since).
		Order("timestamp DESC").
		Find(&dbEvents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query OOM events: %w", err)
	}

	events := make([]types.OOMEvent, 0, len(dbEvents))
	for _, dbEvent := range dbEvents {
		events = append(events, types.OOMEvent{
			ID:                 dbEvent.ID,
			ClusterID:          dbEvent.ClusterID,
			ContainerID:        dbEvent.ContainerID,
			PodName:            dbEvent.PodName,
			NodeName:           dbEvent.NodeName,
			Namespace:          dbEvent.Namespace,
			Timestamp:          dbEvent.Timestamp,
			MemoryLimit:        dbEvent.MemoryLimit,
			MemoryRequest:      dbEvent.MemoryRequest,
			LastObservedMemory: dbEvent.LastObservedMemory,
			CreatedAt:          dbEvent.CreatedAt,
			UpdatedAt:          dbEvent.UpdatedAt,
		})
	}

	return events, nil
}

func (s *GormDB) DeleteOldOOMEvents(clusterID string, olderThan time.Time) (int64, error) {
	result := s.db.Where("cluster_id = ? AND timestamp < ?", clusterID, olderThan).Delete(&OOMEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete old OOM events: %w", result.Error)
	}

	return result.RowsAffected, nil
}

func (s *GormDB) SavePodRecommendations(clusterID string, rows []types.PodResourceRecommendationRow) error {
	if rows == nil {
		return fmt.Errorf("rows cannot be nil")
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cluster_id = ?", clusterID).Delete(&PodResourceRecommendation{}).Error; err != nil {
			return fmt.Errorf("failed to delete pod recommendations for cluster: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		models := make([]PodResourceRecommendation, 0, len(rows))
		for _, r := range rows {
			models = append(models, PodResourceRecommendation{
				ClusterID:      clusterID,
				WorkloadID:     r.WorkloadID,
				NodeName:       r.NodeName,
				Namespace:      r.Namespace,
				Pod:            r.Pod,
				Container:      r.Container,
				Recommendation: r.Recommendation,
			})
		}
		if err := tx.CreateInBatches(models, 100).Error; err != nil {
			return fmt.Errorf("failed to insert pod recommendations: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to save pod recommendations: %w", err)
	}
	return nil
}

func (s *GormDB) GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error) {
	var models []PodResourceRecommendation
	if err := s.db.Where("cluster_id = ?", clusterID).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to get pod recommendations for cluster: %w", err)
	}
	rows := make([]types.PodResourceRecommendationRow, 0, len(models))
	for _, m := range models {
		rows = append(rows, types.PodResourceRecommendationRow{
			WorkloadID:     m.WorkloadID,
			NodeName:       m.NodeName,
			Namespace:      m.Namespace,
			Pod:            m.Pod,
			Container:      m.Container,
			Recommendation: m.Recommendation,
		})
	}
	return rows, nil
}
