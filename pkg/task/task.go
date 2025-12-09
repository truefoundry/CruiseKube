package task

import (
	"context"
)

type Task interface {
	GetName() string
	GetSchedule() string
	IsEnabled() bool
	Run(ctx context.Context) error
	GetCoreTask() any
}
