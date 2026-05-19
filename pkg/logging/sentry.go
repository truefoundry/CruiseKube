package logging

import (
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
)

type sentryReporter struct{}

// buildFingerprint constructs a Sentry fingerprint that groups issues by error
// type (msg) and source (task or api). Events with the same fingerprint are
// collapsed into a single Sentry issue.
func buildFingerprint(msg string, tags map[string]string) []string {
	fingerprint := []string{msg}
	if task := tags["task"]; task != "" {
		fingerprint = append(fingerprint, "task:"+task)
	} else if api := tags["api"]; api != "" {
		fingerprint = append(fingerprint, "api:"+api)
	}
	return fingerprint
}

func (s *sentryReporter) CaptureErrors(errs []error, msg string, tags map[string]string) {
	if len(errs) == 0 {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}

		var combined error
		if len(errs) == 1 {
			combined = errs[0]
		} else {
			combined = errors.Join(errs...)
		}

		event := sentry.NewEvent()
		event.Level = sentry.LevelError
		event.Message = msg
		event.Fingerprint = buildFingerprint(msg, tags)

		event.Exception = []sentry.Exception{
			{
				Type:       msg,
				Value:      combined.Error(),
				Stacktrace: sentry.NewStacktrace(),
			},
		}
		sentry.CaptureEvent(event)
	})
}

func (s *sentryReporter) CaptureMessage(msg string, tags map[string]string) {
	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		event := sentry.NewEvent()
		event.Level = sentry.LevelInfo
		event.Message = msg
		event.Fingerprint = buildFingerprint(msg, tags)
		sentry.CaptureEvent(event)
	})
}

func (s *sentryReporter) Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}

func NewSentryReporter(dsn, environment string) (ErrorReporter, error) {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		AttachStacktrace: true,
	}); err != nil {
		return nil, fmt.Errorf("failed to initialize sentry: %w", err)
	}
	return &sentryReporter{}, nil
}
