package utils

import (
	"context"
	"time"

	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/metrics"
)

func TimeIt(ctx context.Context, name string) func() {
	startTime := time.Now()
	logging.Infof(ctx, "Task %s started", name)
	return func() {
		duration := time.Since(startTime)
		logging.Infof(ctx, "Task %s completed in %v", name, duration)
		clusterID, _ := contextutils.GetCluster(ctx)
		metrics.ObserveOperationDuration(clusterID, name, metrics.StatusComplete, duration)
	}
}
