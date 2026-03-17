package logging

import (
	"context"
	"time"

	"github.com/truefoundry/cruisekube/pkg/contextutils"
)

type ErrorReporter interface {
	CaptureErrors(errs []error, msg string, tags map[string]string)
	CaptureMessage(msg string, tags map[string]string)
	Flush(timeout time.Duration)
}

var errorReporter ErrorReporter

func SetErrorReporter(r ErrorReporter) {
	errorReporter = r
}

func FlushReporter(timeout time.Duration) {
	if errorReporter != nil {
		errorReporter.Flush(timeout)
	}
}

func CaptureToReporter(ctx context.Context, msg string, args ...any) {
	if errorReporter == nil {
		return
	}

	tags := contextutils.GetAttributes(ctx)

	var errs []error
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		errorReporter.CaptureErrors(errs, msg, tags)
	} else {
		errorReporter.CaptureMessage(msg, tags)
	}
}
