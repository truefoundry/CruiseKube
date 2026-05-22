package metricstest

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

func SampleValue(t *testing.T, metricName string, labels map[string]string) float64 {
	t.Helper()

	line, ok := FindSampleLine(t, metricName, labels)
	if !ok {
		return 0
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		t.Fatalf("metric line %q did not include a value", line)
	}

	value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
	if err != nil {
		t.Fatalf("failed to parse metric value from %q: %v", line, err)
	}
	return value
}

func FindSampleLine(t *testing.T, metricName string, labels map[string]string) (string, bool) {
	t.Helper()

	for _, line := range strings.Split(GatherText(t), "\n") {
		if !strings.HasPrefix(line, metricName+"{") && !strings.HasPrefix(line, metricName+" ") {
			continue
		}
		if LabelsMatchLine(line, labels) {
			return line, true
		}
	}
	return "", false
}

func GatherText(t *testing.T) string {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	var buf bytes.Buffer
	for _, family := range families {
		if _, err := expfmt.MetricFamilyToText(&buf, family); err != nil {
			t.Fatalf("failed to encode metrics: %v", err)
		}
	}
	return buf.String()
}

func LabelsMatchLine(line string, labels map[string]string) bool {
	for name, value := range labels {
		if !strings.Contains(line, fmt.Sprintf("%s=%q", name, value)) {
			return false
		}
	}
	return true
}

func UniqueLabel(t *testing.T, prefix string) string {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_", "-", "_").Replace(t.Name())
	return fmt.Sprintf("%s_%s_%d", prefix, name, time.Now().UnixNano())
}
