package task

import (
	"context"
)

type Task interface {
	GetName() string
	GetClusterID() string
	GetSchedule() string
	IsEnabled() bool
	Run(ctx context.Context) error
	GetCoreTask() any
}
