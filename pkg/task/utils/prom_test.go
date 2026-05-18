package utils

import (
	"strings"
	"testing"
)

func TestBuildBatchPodInfoExpressionContainsReadyFilter(t *testing.T) {
	expr := buildBatchPodInfoExpression("test-ns")

	required := []string{
		`kube_pod_status_ready`,
		`condition="true"`,
		`job="kube-state-metrics"`,
		`> 0`,
		`max by (namespace, pod)`,
		`on (namespace, pod)`,
		`group_left`,
	}
	for _, fragment := range required {
		if !strings.Contains(expr, fragment) {
			t.Fatalf("expected %q in expression, got:\n%s", fragment, expr)
		}
	}
	if strings.Count(expr, "test-ns") < 2 {
		t.Fatalf("expected namespace to appear at least twice (pod_info + status_phase), got:\n%s", expr)
	}
}
