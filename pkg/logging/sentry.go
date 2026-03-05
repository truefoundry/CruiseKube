package logging

import (
	"errors"
	"time"

	"github.com/getsentry/sentry-go"
)

type sentryReporter struct{}

func (s *sentryReporter) CaptureErrors(errs []error, msg string, tags map[string]string) {
	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		scope.SetExtra("message", msg)

		var combined error
		if len(errs) == 1 {
			combined = errs[0]
		} else {
			combined = errors.Join(errs...)
		}
		sentry.CaptureException(combined)
	})
}

func (s *sentryReporter) CaptureMessage(msg string, tags map[string]string) {
	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		sentry.CaptureMessage(msg)
	})
}

func (s *sentryReporter) Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}

func NewSentryReporter(dsn, environment string) (ErrorReporter, error) {
	if dsn == "" {
		return nil, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		AttachStacktrace: true,
	}); err != nil {
		return nil, err
	}
	return &sentryReporter{}, nil
}
