package utils

import (
	"context"
	"time"

	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/metrics"
)

func StartTimedOperation(ctx context.Context, name string) func(status string) {
	startTime := time.Now()
	logging.Infof(ctx, "Task %s started", name)
	return func(status string) {
		if status == "" {
			status = metrics.StatusComplete
		}
		duration := time.Since(startTime)
		logging.Infof(ctx, "Task %s completed in %v", name, duration)
		clusterID, _ := contextutils.GetCluster(ctx)
		metrics.ObserveOperationDuration(clusterID, name, status, duration)
	}
}
