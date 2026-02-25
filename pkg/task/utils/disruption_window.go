package utils

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
)

var defaultCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func InEvictionWindow(ctx context.Context, startCron, endCron string, tm time.Time) bool {
	if startCron == "" || endCron == "" {
		return false
	}

	startSchedule, err := defaultCronParser.Parse(startCron)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse disruption window start cron %q: %v", startCron, err)
		return false
	}

	endSchedule, err := defaultCronParser.Parse(endCron)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse disruption window end cron %q: %v", endCron, err)
		return false
	}

	nextStart := startSchedule.Next(tm.UTC())
	nextEnd := endSchedule.Next(tm.UTC())

	return nextEnd.Before(nextStart) || nextEnd.Equal(nextStart)
}

func IsInAnyDisruptionWindow(ctx context.Context, windows []types.DisruptionWindow) bool {
	now := time.Now().UTC()
	for _, w := range windows {
		if InEvictionWindow(ctx, w.StartCron, w.EndCron, now) {
			return true
		}
	}
	return false
}
