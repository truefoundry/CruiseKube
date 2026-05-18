package utils

import (
	"context"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"
)

var defaultCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// normalizeCronDOW7 rewrites day-of-week value 7 (non-standard Sunday alias) to 0
// so that robfig/cron/v3 (which only accepts 0-6) can parse it.
// Note: only comma-separated DOW lists are handled (e.g. "5,7" → "5,0").
// Range expressions like "5-7" are NOT normalized, which is fine because the
// frontend builds cron using comma-separated values via buildCron().
func normalizeCronDOW7(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return expr
	}
	dow := fields[4]
	if dow == "*" {
		return expr
	}
	tokens := strings.Split(dow, ",")
	for i, t := range tokens {
		if t == "7" {
			tokens[i] = "0"
		}
	}
	fields[4] = strings.Join(tokens, ",")
	return strings.Join(fields, " ")
}

func InEvictionWindow(ctx context.Context, startCron, endCron string, tm time.Time) bool {
	if startCron == "" || endCron == "" {
		return false
	}

	startSchedule, err := defaultCronParser.Parse(normalizeCronDOW7(startCron))
	if err != nil {
		logging.Errorf(ctx, "Failed to parse disruption window start cron %q: %v", startCron, err)
		return false
	}

	endSchedule, err := defaultCronParser.Parse(normalizeCronDOW7(endCron))
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
