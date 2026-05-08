package usageheartbeat

import (
	"context"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"k8s.io/client-go/kubernetes"
)

// Start runs heartbeat ticks until ctx is cancelled. Errors are logged; always returns nil so the runtime goroutine does not fail the process.
func Start(ctx context.Context, kube kubernetes.Interface, cfg *config.Config, reporter ports.UsageTelemetryReporter) error {
	interval, err := time.ParseDuration(strings.TrimSpace(cfg.UsageTelemetry.Interval))
	if err != nil {
		logging.Errorf(ctx, "usage heartbeat: invalid interval: %v", err)
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	send := func() {
		hb, err := Collect(ctx, kube, cfg)
		if err != nil {
			logging.Warnf(ctx, "usage heartbeat: collect failed: %v", err)
			return
		}
		hb.CruisekubeVersion = buildmetadata.Version
		if err := reporter.ReportHeartbeat(ctx, hb); err != nil {
			logging.Warnf(ctx, "usage heartbeat: report failed: %v", err)
		}
	}
	send()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			send()
		}
	}
}
