package logging

import (
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
)

type sentryReporter struct{}

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
		// Group by call stack so errors from the same code location are clustered
		// together even when the message contains dynamic values (workload names, etc.).
		event.Fingerprint = []string{"{{ stack }}"}

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
		scope.SetFingerprint([]string{"{{ stack }}"})
		sentry.CaptureMessage(msg)
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
