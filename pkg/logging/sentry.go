package logging

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

type sentryReporter struct{}

func (s *sentryReporter) CaptureErrors(errs []error, msg string, fingerprint []string, tags map[string]string) {
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
		event.Fingerprint = fingerprint
		event.Extra = map[string]interface{}{
			"original_message": msg,
		}
		event.Exception = []sentry.Exception{
			{
				Type:       strings.Join(fingerprint, " | "),
				Value:      combined.Error(),
				Stacktrace: sentry.NewStacktrace(),
			},
		}
		sentry.CaptureEvent(event)
	})
}

func (s *sentryReporter) CaptureMessage(msg string, fingerprint []string, tags map[string]string) {
	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		scope.SetFingerprint(fingerprint)
		scope.SetExtra("original_message", msg)
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
		BeforeSend:       normalizeSentryEvent,
	}); err != nil {
		return nil, fmt.Errorf("failed to initialize sentry: %w", err)
	}
	return &sentryReporter{}, nil
}

// buildFingerprint builds a source-aware fingerprint for Sentry grouping.
// callerSkip is the number of frames to skip beyond buildFingerprint itself.
// Example call stacks:
//
//	runtime.Caller(2) <- buildFingerprint <- Errorf <- actualCaller  (callerSkip=1)
//	runtime.Caller(2) <- buildFingerprint <- Error  <- actualCaller  (callerSkip=1)
func buildFingerprint(format string, callerSkip int) []string {
	pc, file, _, ok := runtime.Caller(callerSkip + 1)
	if !ok {
		return []string{normalizeMessage(format)}
	}
	// Extract short file path (last 3 components, e.g. "pkg/task/taskapplyrecommendation.go")
	parts := strings.Split(file, "/")
	if len(parts) > 3 {
		file = strings.Join(parts[len(parts)-3:], "/")
	}
	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fullName := fn.Name()
		// Extract just the function name after the last "/"
		if idx := strings.LastIndex(fullName, "/"); idx >= 0 {
			funcName = fullName[idx+1:]
		} else {
			funcName = fullName
		}
	}
	return []string{file, funcName, normalizeMessage(format)}
}

var dynamicPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), // UUIDs (case-insensitive, word boundaries)
	regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b`),                          // IPv4 addresses with optional port
	regexp.MustCompile(`\b\d{6,}\b`),                                                                // Numeric IDs (6+ digits)
}

func normalizeMessage(msg string) string {
	result := msg
	for _, re := range dynamicPatterns {
		result = re.ReplaceAllString(result, "<*>")
	}
	return result
}

func normalizeSentryEvent(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	for i := range event.Exception {
		event.Exception[i].Type = normalizeMessage(event.Exception[i].Type)
	}
	event.Message = normalizeMessage(event.Message)
	for i := range event.Fingerprint {
		event.Fingerprint[i] = normalizeMessage(event.Fingerprint[i])
	}
	return event
}
