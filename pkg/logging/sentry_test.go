package logging

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *capturingTransport) Configure(options sentry.ClientOptions)    {}
func (t *capturingTransport) Close()                                    {}
func (t *capturingTransport) FlushWithContext(ctx context.Context) bool { return true }
func (t *capturingTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}
func (t *capturingTransport) Flush(timeout time.Duration) bool { return true }
func (t *capturingTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]*sentry.Event{}, t.events...)
}

func setupTestSentry(t *testing.T) *capturingTransport {
	transport := &capturingTransport{}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:        "https://key@sentry.io/1",
		Transport:  transport,
		BeforeSend: normalizeSentryEvent, // shared with production
	})
	require.NoError(t, err)
	SetErrorReporter(&sentryReporter{})
	t.Cleanup(func() {
		sentry.Flush(time.Second)
		SetErrorReporter(nil)
	})
	return transport
}

func TestNormalizeMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no dynamic content",
			input:    "Error evicting pod ns-a/pod-abc: timeout",
			expected: "Error evicting pod ns-a/pod-abc: timeout",
		},
		{
			name:     "UUID",
			input:    "Failed for UUID 550e8400-e29b-41d4-a716-446655440000",
			expected: "Failed for UUID <*>",
		},
		{
			name:     "IPv4 with port",
			input:    "Connection to 192.168.1.1:8080 failed",
			expected: "Connection to <*> failed",
		},
		{
			name:     "numeric ID 6+ digits",
			input:    "Failed to unmarshal audit payload id: 1234567: err",
			expected: "Failed to unmarshal audit payload id: <*>: err",
		},
		{
			name:     "format verbs unchanged",
			input:    "Error evicting pod %s/%s: %v",
			expected: "Error evicting pod %s/%s: %v",
		},
		{
			name:     "static message unchanged",
			input:    "Static error message",
			expected: "Static error message",
		},
		{
			name:     "uppercase UUID",
			input:    "Failed for UUID 550E8400-E29B-41D4-A716-446655440000",
			expected: "Failed for UUID <*>",
		},
		{
			name:     "mixed UUID and IP",
			input:    "Mixed 550e8400-e29b-41d4-a716-446655440000 and 10.0.0.1",
			expected: "Mixed <*> and <*>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMessage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildFingerprint(t *testing.T) {
	fp := buildFingerprint("Error %s", 0)
	require.Len(t, fp, 3, "fingerprint should have 3 elements")
	assert.NotEmpty(t, fp[0], "file component should be non-empty")
	assert.NotEmpty(t, fp[1], "function component should be non-empty")
	assert.Contains(t, fp[0], "sentry_test.go", "file should contain test file name")
	assert.Contains(t, fp[1], "TestBuildFingerprint", "function should contain test function name")
	assert.Equal(t, "Error %s", fp[2], "format component should be the normalized format string")
}

func TestCaptureErrorsFingerprint(t *testing.T) {
	transport := setupTestSentry(t)

	reporter := &sentryReporter{}
	fingerprint := []string{"file.go", "myFunc", "Error %s"}
	reporter.CaptureErrors(
		[]error{fmt.Errorf("something failed")},
		"Error pod-1",
		fingerprint,
		map[string]string{"env": "test"},
	)
	sentry.Flush(time.Second)

	events := transport.Events()
	require.Len(t, events, 1)
	event := events[0]

	assert.Equal(t, fingerprint, event.Fingerprint)
	assert.Equal(t, "Error pod-1", event.Extra["original_message"])
	assert.Equal(t, "something failed", event.Exception[0].Value)
	assert.Equal(t, sentry.LevelError, event.Level)
}

func TestCaptureMessageFingerprint(t *testing.T) {
	transport := setupTestSentry(t)

	reporter := &sentryReporter{}
	fingerprint := []string{"file.go", "myFunc", "Static message"}
	reporter.CaptureMessage("Static message", fingerprint, map[string]string{})
	sentry.Flush(time.Second)

	events := transport.Events()
	require.Len(t, events, 1)
	event := events[0]

	assert.Equal(t, "Static message", event.Message)
	assert.Equal(t, fingerprint, event.Fingerprint)
}

func TestBeforeSendNormalizesFingerprint(t *testing.T) {
	event := &sentry.Event{
		Fingerprint: []string{"file.go", "myFunc", "Error 550e8400-e29b-41d4-a716-446655440000"},
		Message:     "Error 550e8400-e29b-41d4-a716-446655440000",
		Exception: []sentry.Exception{
			{Type: "Error 550e8400-e29b-41d4-a716-446655440000"},
		},
	}
	result := normalizeSentryEvent(event, nil)

	assert.Equal(t, "Error <*>", result.Fingerprint[2])
	assert.Equal(t, "Error <*>", result.Message)
	assert.Equal(t, "Error <*>", result.Exception[0].Type)
}

func TestSameFormatDifferentArgsSameFingerprint(t *testing.T) {
	transport := setupTestSentry(t)

	ctx := context.Background()
	Errorf(ctx, "Error evicting pod %s/%s: %v", "ns-a", "pod-1", fmt.Errorf("timeout"))
	Errorf(ctx, "Error evicting pod %s/%s: %v", "ns-b", "pod-2", fmt.Errorf("deadline exceeded"))
	sentry.Flush(time.Second)

	events := transport.Events()
	require.Len(t, events, 2)
	assert.Equal(t, events[0].Fingerprint, events[1].Fingerprint,
		"same format string from same call site should produce same fingerprint")
}

//go:noinline
func wrapperA(ctx context.Context) {
	Errorf(ctx, "Error evicting pod %s/%s: %v", "ns-a", "pod-1", fmt.Errorf("timeout"))
}

//go:noinline
func wrapperB(ctx context.Context) {
	Errorf(ctx, "Error evicting pod %s/%s: %v", "ns-b", "pod-2", fmt.Errorf("deadline exceeded"))
}

func TestDifferentCallSitesSameFormatDifferentFingerprint(t *testing.T) {
	transport := setupTestSentry(t)

	ctx := context.Background()
	wrapperA(ctx)
	wrapperB(ctx)
	sentry.Flush(time.Second)

	events := transport.Events()
	require.Len(t, events, 2)
	assert.NotEqual(t, events[0].Fingerprint, events[1].Fingerprint,
		"different call sites should produce different fingerprints")
}

func TestErrorFunctionGeneratesFingerprint(t *testing.T) {
	transport := setupTestSentry(t)

	ctx := context.Background()
	Error(ctx, "Static error message", fmt.Errorf("some error"))
	sentry.Flush(time.Second)

	events := transport.Events()
	require.Len(t, events, 1)
	event := events[0]

	require.Len(t, event.Fingerprint, 3, "fingerprint should have 3 elements")
	assert.Contains(t, event.Fingerprint[0], "sentry_test.go", "file component should contain test file name")
	assert.Equal(t, "Static error message", event.Fingerprint[2], "format component should be the static msg")
}

func TestTagsArePreserved(t *testing.T) {
	transport := setupTestSentry(t)

	reporter := &sentryReporter{}
	tags := map[string]string{"service": "cruisekube", "env": "test"}
	reporter.CaptureErrors(
		[]error{fmt.Errorf("fail")},
		"error msg",
		[]string{"file.go", "func", "error msg"},
		tags,
	)
	sentry.Flush(time.Second)

	events := transport.Events()
	require.Len(t, events, 1)
	event := events[0]

	assert.Equal(t, "cruisekube", event.Tags["service"])
	assert.Equal(t, "test", event.Tags["env"])
}
