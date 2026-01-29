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

type CleanupOOMEventsTaskConfig struct {
	Name      string
	Enabled   bool
	Schedule  string
	ClusterID string
	Metadata  CleanupOOMEventsMetadata
}

type CleanupOOMEventsTask struct {
	config  *CleanupOOMEventsTaskConfig
	storage *storage.Storage
}

type CleanupOOMEventsMetadata struct {
	RetentionDays int `mapstructure:"retentionDays"`
}

func NewCleanupOOMEventsTask(ctx context.Context, storage *storage.Storage, config *CleanupOOMEventsTaskConfig, taskConfig *config.TaskConfig) *CleanupOOMEventsTask {
	var cleanupOOMEventsMetadata CleanupOOMEventsMetadata
	if err := taskConfig.ConvertMetadataToStruct(&cleanupOOMEventsMetadata); err != nil {
		logging.Errorf(ctx, "Error converting metadata to struct: %v", err)
		return nil
	}

	if cleanupOOMEventsMetadata.RetentionDays <= 0 {
		cleanupOOMEventsMetadata.RetentionDays = DefaultRetentionDays
	}

	config.Metadata = cleanupOOMEventsMetadata
	return &CleanupOOMEventsTask{
		config:  config,
		storage: storage,
	}
}

func (t *CleanupOOMEventsTask) GetCoreTask() any {
	return t
}

func (t *CleanupOOMEventsTask) GetName() string {
	return t.config.Name
}

func (t *CleanupOOMEventsTask) GetSchedule() string {
	return t.config.Schedule
}

func (t *CleanupOOMEventsTask) IsEnabled() bool {
	return t.config.Enabled
}

func (t *CleanupOOMEventsTask) Run(ctx context.Context) error {
	ctx = contextutils.WithTask(ctx, t.config.Name)
	ctx = contextutils.WithCluster(ctx, t.config.ClusterID)

	deletedCount, err := t.storage.DeleteOldOOMEvents(t.config.ClusterID, t.config.Metadata.RetentionDays)
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
