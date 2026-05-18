package prometheus

import (
	"strings"
	"testing"
)

func TestBuildBatchPodInfoExpressionContainsPhaseFilter(t *testing.T) {
	p := &PrometheusProvider{}
	expr := p.buildBatchPodInfoExpression("test-ns")

	required := []string{
		`kube_pod_status_phase`,
		`phase="Running"`,
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

func TestBuildBatchReplicaCountQueryContainsPhaseFilter(t *testing.T) {
	p := &PrometheusProvider{}
	query := p.buildBatchReplicaCountQuery("test-ns")

	required := []string{
		`kube_pod_status_phase`,
		`phase="Running"`,
		`job="kube-state-metrics"`,
		`> 0`,
	}
	for _, fragment := range required {
		if !strings.Contains(query, fragment) {
			t.Fatalf("expected %q in replica count query, got:\n%s", fragment, query)
		}
	}
}
