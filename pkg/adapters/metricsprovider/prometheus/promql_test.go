package prometheus

import (
	"strings"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/metrics"
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

func TestQueryKindFromID(t *testing.T) {
	tests := []struct {
		name     string
		queryID  string
		expected string
	}{
		{name: "psi check", queryID: "PSI_CHECK", expected: "psi_check"},
		{name: "node load monitoring", queryID: "node-load-monitoring", expected: "node_load_monitoring"},
		{name: "cluster cpu utilization", queryID: "cluster_cpu_utilization", expected: "cluster_cpu_utilization"},
		{name: "cluster cpu request", queryID: "cluster_cpu_request", expected: "cluster_cpu_request"},
		{name: "cluster cpu allocated", queryID: "cluster_cpu_allocated", expected: "cluster_cpu_allocated"},
		{name: "cluster memory utilization", queryID: "cluster_memory_utilization", expected: "cluster_memory_utilization"},
		{name: "cluster memory request", queryID: "cluster_memory_request", expected: "cluster_memory_request"},
		{name: "cluster memory allocated", queryID: "cluster_memory_allocated", expected: "cluster_memory_allocated"},
		{name: "cluster oom events", queryID: "cluster_oom_events", expected: "cluster_oom_events"},
		{name: "karpenter consolidation", queryID: "cluster_unschedulable_pods", expected: "cluster_unschedulable_pods"},
		{name: "unschedulable pods", queryID: "cluster_unschedulable_pods_count", expected: "cluster_unschedulable_pods_count"},
		{name: "node cpu waiting max", queryID: "node_cpu_waiting_max", expected: "node_cpu_waiting_max"},
		{name: "node cpu waiting p50", queryID: "node_cpu_waiting_p50", expected: "node_cpu_waiting_p50"},
		{name: "node cpu load max", queryID: "node_cpu_load_max", expected: "node_cpu_load_max"},
		{name: "node cpu load p50", queryID: "node_cpu_load_p50", expected: "node_cpu_load_p50"},
		{name: "container cpu waiting max", queryID: "container_cpu_waiting_max", expected: "container_cpu_waiting_max"},
		{name: "container cpu waiting p50", queryID: "container_cpu_waiting_p50", expected: "container_cpu_waiting_p50"},
		{name: "node memory waiting max", queryID: "node_memory_waiting_max", expected: "node_memory_waiting_max"},
		{name: "node memory waiting p50", queryID: "node_memory_waiting_p50", expected: "node_memory_waiting_p50"},
		{name: "container memory waiting max", queryID: "container_memory_waiting_max", expected: "container_memory_waiting_max"},
		{name: "container memory waiting p50", queryID: "container_memory_waiting_p50", expected: "container_memory_waiting_p50"},
		{name: "namespace cpu p75", queryID: "foo-cpu_p75", expected: "namespace_cpu_p75"},
		{name: "namespace cpu p75 psi", queryID: "foo-cpu_p75-psi", expected: "namespace_cpu_p75_psi"},
		{name: "namespace memory p75", queryID: "foo-memory_p75-memory", expected: "namespace_memory_p75"},
		{name: "namespace memory max 7 day", queryID: "foo-memory_max-memory-7day", expected: "namespace_memory_max_7day"},
		{name: "namespace replica", queryID: "foo-replica", expected: "namespace_replica"},
		{name: "unmatched intentionally bounded", queryID: "custom-debug-query", expected: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryKindFromID(tt.queryID); got != tt.expected {
				t.Fatalf("queryKindFromID(%q) = %q, want %q", tt.queryID, got, tt.expected)
			}
		})
	}
}

func TestNamespaceQueryRequestCarriesQueryKind(t *testing.T) {
	request := namespaceQueryRequest("default", "cpu_p75", "up", metrics.PrometheusQueryKindNamespaceCPUP75)

	if request.QueryID != "default-cpu_p75" {
		t.Fatalf("QueryID = %q, want %q", request.QueryID, "default-cpu_p75")
	}
	if request.Query != "up" {
		t.Fatalf("Query = %q, want %q", request.Query, "up")
	}
	if request.QueryKind != metrics.PrometheusQueryKindNamespaceCPUP75 {
		t.Fatalf("QueryKind = %q, want %q", request.QueryKind, metrics.PrometheusQueryKindNamespaceCPUP75)
	}
}
