package task

import (
	"context"
	"fmt"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
)

const DefaultRetentionDays = 7

type CleanupTaskConfig struct {
	Name      string
	Enabled   bool
	Schedule  string
	ClusterID string
	Metadata  CleanupMetadata
}

type CleanupTask struct {
	config  *CleanupTaskConfig
	storage *storage.Storage
}

type CleanupMetadata struct {
	OOMEventsRetentionDays   int `mapstructure:"oomEventsRetentionDays"`
	AuditEventsRetentionDays int `mapstructure:"auditEventsRetentionDays"`
	SnapshotsRetentionDays   int `mapstructure:"snapshotsRetentionDays"`
}

func NewCleanupTask(ctx context.Context, storage *storage.Storage, config *CleanupTaskConfig, taskConfig *config.TaskConfig) *CleanupTask {
	var metadata CleanupMetadata
	if err := taskConfig.ConvertMetadataToStruct(&metadata); err != nil {
		logging.Errorf(ctx, "Error converting metadata to struct: %v", err)
		return nil
	}

	if metadata.OOMEventsRetentionDays <= 0 {
		metadata.OOMEventsRetentionDays = DefaultRetentionDays
	}
	if metadata.AuditEventsRetentionDays <= 0 {
		metadata.AuditEventsRetentionDays = DefaultRetentionDays
	}
	if metadata.SnapshotsRetentionDays <= 0 {
		metadata.SnapshotsRetentionDays = DefaultRetentionDays
	}

	config.Metadata = metadata
	return &CleanupTask{
		config:  config,
		storage: storage,
	}
}

func (t *CleanupTask) GetCoreTask() any {
	return t
}

func (t *CleanupTask) GetName() string {
	return t.config.Name
}

func (t *CleanupTask) GetSchedule() string {
	return t.config.Schedule
}

func (t *CleanupTask) IsEnabled() bool {
	return t.config.Enabled
}

func (t *CleanupTask) Run(ctx context.Context) error {
	ctx = contextutils.WithTask(ctx, t.config.Name)
	ctx = contextutils.WithCluster(ctx, t.config.ClusterID)

	if err := t.cleanupOOMEvents(ctx); err != nil {
		return err
	}
	if err := t.cleanupAuditEvents(ctx); err != nil {
		return err
	}
	if err := t.cleanupSnapshots(ctx); err != nil {
		return err
	}

	return nil
}

func (t *CleanupTask) cleanupOOMEvents(ctx context.Context) error {
	deletedCount, err := t.storage.DeleteOldOOMEvents(t.config.ClusterID, t.config.Metadata.OOMEventsRetentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old OOM events: %w", err)
	}

	if deletedCount > 0 {
		logging.Infof(ctx, "Successfully deleted %d old OOM events", deletedCount)
	} else {
		logging.Debugf(ctx, "No old OOM events to cleanup")
	}
	return nil
}

func (t *CleanupTask) cleanupAuditEvents(ctx context.Context) error {
	deletedCount, err := t.storage.DeleteOldAuditEvents(t.config.ClusterID, t.config.Metadata.AuditEventsRetentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old audit events: %w", err)
	}

	if deletedCount > 0 {
		logging.Infof(ctx, "Successfully deleted %d old audit events", deletedCount)
	} else {
		logging.Debugf(ctx, "No old audit events to cleanup")
	}
	return nil
}

func (t *CleanupTask) cleanupSnapshots(ctx context.Context) error {
	deletedCount, err := t.storage.DeleteOldSnapshots(t.config.ClusterID, t.config.Metadata.SnapshotsRetentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old snapshots: %w", err)
	}

	if deletedCount > 0 {
		logging.Infof(ctx, "Successfully deleted %d old snapshots", deletedCount)
	} else {
		logging.Debugf(ctx, "No old snapshots to cleanup")
	}
	return nil
}
