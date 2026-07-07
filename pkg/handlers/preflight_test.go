package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	apiversion "k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// stubPromAPI is a minimal promAPI implementation for tests.
type stubPromAPI struct {
	buildinfo      promv1.BuildinfoResult
	buildinfoErr   error
	queryFunc      func(query string) (model.Value, error)
	labelNamesFunc func(matches []string) ([]string, error)
}

func (s *stubPromAPI) Buildinfo(_ context.Context) (promv1.BuildinfoResult, error) {
	return s.buildinfo, s.buildinfoErr
}

func (s *stubPromAPI) Query(_ context.Context, query string, _ time.Time, _ ...promv1.Option) (model.Value, promv1.Warnings, error) {
	if s.queryFunc == nil {
		return model.Vector{}, nil, nil
	}
	v, err := s.queryFunc(query)
	return v, nil, err
}

func (s *stubPromAPI) LabelNames(_ context.Context, matches []string, _, _ time.Time, _ ...promv1.Option) ([]string, promv1.Warnings, error) {
	if s.labelNamesFunc == nil {
		return nil, nil, nil
	}
	names, err := s.labelNamesFunc(matches)
	return names, nil, err
}

// countVector returns a Prometheus count() result carrying n.
func countVector(n int) model.Vector {
	return model.Vector{&model.Sample{Value: model.SampleValue(n)}}
}

func mustVer(t *testing.T, s string) *version.Version {
	t.Helper()
	v, err := version.ParseGeneric(s)
	if err != nil {
		t.Fatalf("parse version %q: %v", s, err)
	}
	return v
}

func node(name, kubelet string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          kubelet,
				KubeProxyVersion:        kubelet,
				OSImage:                 "Ubuntu 22.04",
				ContainerRuntimeVersion: "containerd://1.7.0",
				KernelVersion:           "5.15.0",
				Architecture:            "amd64",
			},
		},
	}
}

func TestCollectNodeVersions(t *testing.T) {
	client := fake.NewSimpleClientset(node("node-b", "v1.28.2"), node("node-a", "v1.20.0"))
	infos, below, err := collectNodeVersions(context.Background(), client, mustVer(t, "v1.24.0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if below != 1 {
		t.Fatalf("expected 1 node below minimum, got %d", below)
	}
	// Sorted by name: node-a (below) then node-b (ok).
	if infos[0].Name != "node-a" || infos[0].MeetsMinimum {
		t.Fatalf("node-a should be below minimum: %+v", infos[0])
	}
	if infos[1].Name != "node-b" || !infos[1].MeetsMinimum {
		t.Fatalf("node-b should meet minimum: %+v", infos[1])
	}
}

func TestCollectNodeVersionsNilClient(t *testing.T) {
	if _, _, err := collectNodeVersions(context.Background(), nil, mustVer(t, "v1.24.0")); err == nil {
		t.Fatal("expected error for nil kube client")
	}
}

func TestBuildVersionReportSetsCruisekubeVersion(t *testing.T) {
	client := fake.NewSimpleClientset()
	rep := buildVersionReport(context.Background(), client, mustVer(t, "v1.34.0"), mustVer(t, "v1.34.0"), mustVer(t, "2.30.0"))
	if rep.CruisekubeVersion == "" {
		t.Fatal("cruisekube_version should be populated (buildmetadata.Version)")
	}
}

func TestDetectKubernetesServerVersion(t *testing.T) {
	client := fake.NewSimpleClientset()
	fd := client.Discovery().(*fakediscovery.FakeDiscovery)

	fd.FakedServerVersion = &apiversion.Info{GitVersion: "v1.34.2"}
	got := detectKubernetesServerVersion(client, mustVer(t, "v1.34.0"))
	if !got.MeetsMinimum || got.Version != "v1.34.2" {
		t.Fatalf("v1.34.2 should meet minimum 1.34.0: %+v", got)
	}

	fd.FakedServerVersion = &apiversion.Info{GitVersion: "v1.30.0"}
	got = detectKubernetesServerVersion(client, mustVer(t, "v1.34.0"))
	if got.MeetsMinimum {
		t.Fatalf("v1.30.0 should be below 1.34.0: %+v", got)
	}

	if got := detectKubernetesServerVersion(nil, mustVer(t, "v1.34.0")); got.Error == "" {
		t.Fatal("nil client should report an error")
	}
}

func TestEvaluatePrometheusVersion(t *testing.T) {
	min := mustVer(t, "2.30.0")
	if got := evaluatePrometheusVersion("2.45.0", min); !got.MeetsMinimum {
		t.Fatalf("2.45.0 should meet 2.30.0: %+v", got)
	}
	if got := evaluatePrometheusVersion("2.10.0", min); got.MeetsMinimum {
		t.Fatalf("2.10.0 should not meet 2.30.0: %+v", got)
	}
	if got := evaluatePrometheusVersion("", min); got.Error == "" {
		t.Fatal("empty version should report an error")
	}
}

func TestSplitPrometheusTarget(t *testing.T) {
	cases := map[string][2]string{
		"http://prom.monitoring:9090": {"prom.monitoring", "9090"},
		"https://prom.example.com":    {"prom.example.com", "443"},
		"http://prom.example.com":     {"prom.example.com", "80"},
	}
	for in, want := range cases {
		host, port := splitPrometheusTarget(in)
		if host != want[0] || port != want[1] {
			t.Fatalf("%s => (%q,%q), want (%q,%q)", in, host, port, want[0], want[1])
		}
	}
}

func TestCheckPrometheusConnectivityBuildinfo(t *testing.T) {
	client := &stubPromAPI{buildinfo: promv1.BuildinfoResult{Version: "2.45.0", Revision: "abc"}}
	conn := checkPrometheusConnectivity(context.Background(), client, &cluster.PrometheusConnectionInfo{URL: "http://prom:9090"})
	if !conn.Connected || !conn.Healthy {
		t.Fatalf("expected connected+healthy: %+v", conn)
	}
	if conn.Probe != "buildinfo" || conn.Version != "2.45.0" {
		t.Fatalf("expected buildinfo probe with version: %+v", conn)
	}
	if conn.Target != "http://prom:9090" || conn.URL != "http://prom:9090" {
		t.Fatalf("expected target/url set: %+v", conn)
	}
	if conn.Host != "prom" || conn.Port != "9090" {
		t.Fatalf("expected host/port parsed: %+v", conn)
	}
}

func TestCheckPrometheusConnectivityFallback(t *testing.T) {
	client := &stubPromAPI{
		buildinfoErr: context.DeadlineExceeded,
		queryFunc:    func(string) (model.Value, error) { return countVector(1), nil },
	}
	conn := checkPrometheusConnectivity(context.Background(), client, &cluster.PrometheusConnectionInfo{URL: "http://prom:9090"})
	if !conn.Connected || conn.Probe != "query-fallback" {
		t.Fatalf("expected query-fallback connect: %+v", conn)
	}
}

func TestCheckPrometheusConnectivityUnreachable(t *testing.T) {
	client := &stubPromAPI{
		buildinfoErr: context.DeadlineExceeded,
		queryFunc:    func(string) (model.Value, error) { return nil, context.DeadlineExceeded },
	}
	conn := checkPrometheusConnectivity(context.Background(), client, &cluster.PrometheusConnectionInfo{URL: "http://prom:9090"})
	if conn.Connected {
		t.Fatalf("expected not connected: %+v", conn)
	}
	if !strings.Contains(conn.Error, "http://prom:9090") {
		t.Fatalf("error should carry the target URL: %q", conn.Error)
	}
}

func TestProbeMetricsAllPresent(t *testing.T) {
	client := &stubPromAPI{
		queryFunc:      func(string) (model.Value, error) { return countVector(3), nil },
		labelNamesFunc: func([]string) ([]string, error) { return []string{"__name__", "job", "namespace", "pod"}, nil },
	}
	report := probeMetrics(context.Background(), client, "15m", "")
	if !report.Passed {
		t.Fatalf("expected all metrics present, report=%+v", report)
	}
	for _, g := range report.Groups {
		if !g.Present {
			t.Fatalf("group %s should be present", g.Name)
		}
		for _, ck := range g.Checks {
			// __name__ must be stripped; real dimensions preserved.
			if len(ck.Labels) != 3 {
				t.Fatalf("check %s expected 3 labels (no __name__), got %v", ck.Metric, ck.Labels)
			}
			for _, l := range ck.Labels {
				if l == "__name__" {
					t.Fatalf("check %s should not include __name__", ck.Metric)
				}
			}
		}
	}
}

// TestPreflightResponseSendsAllMetricsWithLabels proves that every probed metric
// appears in the serialized JSON and that each present metric carries its
// distinct label names on the wire.
func TestPreflightResponseSendsAllMetricsWithLabels(t *testing.T) {
	client := &stubPromAPI{
		queryFunc:      func(string) (model.Value, error) { return countVector(5), nil },
		labelNamesFunc: func([]string) ([]string, error) { return []string{"__name__", "job", "namespace", "pod"}, nil },
	}
	report := probeMetrics(context.Background(), client, "15m", "")

	// Every probe must be represented in the report.
	got := 0
	for _, g := range report.Groups {
		got += len(g.Checks)
	}
	if got != len(preflightMetricProbes) {
		t.Fatalf("expected %d metric checks in report, got %d", len(preflightMetricProbes), got)
	}

	// Serialize as the API would and confirm labels reach the wire for present metrics.
	blob, err := json.Marshal(PreflightResponse{Metrics: report})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Metrics struct {
			Groups []struct {
				Checks []struct {
					Metric  string   `json:"metric"`
					Present bool     `json:"present"`
					Labels  []string `json:"labels"`
				} `json:"checks"`
			} `json:"groups"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	seen := 0
	for _, g := range decoded.Metrics.Groups {
		for _, ck := range g.Checks {
			seen++
			if !ck.Present {
				t.Fatalf("metric %s should be present", ck.Metric)
			}
			want := []string{"job", "namespace", "pod"} // __name__ stripped
			if len(ck.Labels) != len(want) {
				t.Fatalf("metric %s: expected labels %v on the wire, got %v", ck.Metric, want, ck.Labels)
			}
		}
	}
	if seen != len(preflightMetricProbes) {
		t.Fatalf("expected %d metrics serialized, got %d", len(preflightMetricProbes), seen)
	}
}

func TestMetricLabelNamesStripsName(t *testing.T) {
	client := &stubPromAPI{labelNamesFunc: func([]string) ([]string, error) {
		return []string{"__name__", "instance", "node"}, nil
	}}
	labels, err := metricLabelNames(context.Background(), client, `node_load1{job="node-exporter"}`, "15m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 2 || labels[0] != "instance" || labels[1] != "node" {
		t.Fatalf("expected [instance node], got %v", labels)
	}
}

func TestProbeMetricsMissingOne(t *testing.T) {
	client := &stubPromAPI{queryFunc: func(q string) (model.Value, error) {
		if strings.Contains(q, "node_load1") {
			return model.Vector{}, nil // absent
		}
		return countVector(1), nil
	}}
	report := probeMetrics(context.Background(), client, "15m", "")
	if report.Passed {
		t.Fatal("expected report to fail when node_load1 is missing")
	}
	var found bool
	for _, g := range report.Groups {
		if g.Name == "node-exporter" {
			if g.Present {
				t.Fatal("node-exporter group should not be present")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("node-exporter group missing from report")
	}
}

// TestProbeMetricsIncludesAbsentMetrics proves that ALL probed metrics are in the
// report whether found or not, each with full info (present/series/labels/error),
// and that labels serialize as [] (never null) even for absent metrics.
func TestProbeMetricsIncludesAbsentMetrics(t *testing.T) {
	client := &stubPromAPI{
		queryFunc: func(q string) (model.Value, error) {
			if strings.Contains(q, "node_load1") {
				return model.Vector{}, nil // absent
			}
			return countVector(2), nil
		},
		labelNamesFunc: func([]string) ([]string, error) { return []string{"__name__", "job", "instance"}, nil },
	}
	report := probeMetrics(context.Background(), client, "15m", "")

	// Flatten and index by metric name.
	byMetric := map[string]MetricCheck{}
	for _, g := range report.Groups {
		for _, ck := range g.Checks {
			byMetric[ck.Metric] = ck
		}
	}

	// Every probed metric must be present in the report, found or not.
	for _, p := range preflightMetricProbes {
		ck, ok := byMetric[p.Metric]
		if !ok {
			t.Fatalf("metric %s missing from report", p.Metric)
		}
		if ck.Labels == nil {
			t.Fatalf("metric %s: labels must be non-nil (serialize as []), got nil", p.Metric)
		}
	}

	// The absent metric is still reported, with full info and an error.
	absent := byMetric["node_load1"]
	if absent.Present || len(absent.Labels) != 0 || absent.Error == "" {
		t.Fatalf("absent node_load1 should be present=false, labels=[], error!='' : %+v", absent)
	}
	// A found metric carries its labels.
	found := byMetric["kube_pod_info"]
	if !found.Present || len(found.Labels) != 2 { // __name__ stripped
		t.Fatalf("found kube_pod_info should be present with 2 labels: %+v", found)
	}

	// On the wire: no null labels anywhere.
	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), `"labels":null`) {
		t.Fatalf("labels serialized as null; expected []: %s", blob)
	}
}

func TestProbeMetricsConnErr(t *testing.T) {
	report := probeMetrics(context.Background(), nil, "15m", "boom")
	if report.Passed {
		t.Fatal("expected failure when connectivity error is set")
	}
	for _, g := range report.Groups {
		for _, ck := range g.Checks {
			if ck.Present || ck.Error != "boom" {
				t.Fatalf("check %s should carry conn error: %+v", ck.Metric, ck)
			}
			if ck.Labels == nil {
				t.Fatalf("check %s labels must be a non-nil slice (serializes as []), got nil", ck.Metric)
			}
		}
	}

	// Verify on the wire: labels must be [] for absent metrics, never null.
	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), `"labels":null`) {
		t.Fatalf("labels serialized as null; expected []: %s", blob)
	}
}

func TestFinalizePreflightHealthy(t *testing.T) {
	resp := &PreflightResponse{
		PrometheusConnectivity: PrometheusConnectivity{Connected: true, Healthy: true},
		Versions: VersionReport{
			MinKubeVersion: "1.24.0", MinKubernetesVersion: "1.34.0", MinPrometheusVersion: "2.30.0",
			Kubernetes: KubernetesVersionInfo{Version: "v1.34.2", MeetsMinimum: true},
			Nodes:      []NodeVersionInfo{{Name: "n1", MeetsMinimum: true}},
			NodeCount:  1,
			Prometheus: PrometheusVersionInfo{Version: "2.45.0", MeetsMinimum: true},
		},
		Metrics: MetricReport{Passed: true, Groups: []MetricGroup{
			{Name: "kube-state-metrics", Present: true, Checks: []MetricCheck{{Metric: "kube_pod_info", Present: true}}},
		}},
	}
	resp.Versions.finalize()
	finalizePreflight(resp)
	if !resp.Healthy {
		t.Fatalf("expected healthy, got %+v", resp)
	}
	if len(resp.Failures) != 0 {
		t.Fatalf("expected no failures, got %+v", resp.Failures)
	}
	if resp.Summary.Failed != 0 || resp.Summary.Passed != resp.Summary.TotalChecks {
		t.Fatalf("summary mismatch: %+v", resp.Summary)
	}
}

func TestFinalizePreflightUnhealthy(t *testing.T) {
	resp := &PreflightResponse{
		PrometheusConnectivity: PrometheusConnectivity{Connected: false, Healthy: false, Error: "unreachable at http://prom:9090"},
		Versions: VersionReport{
			MinKubeVersion: "1.24.0", MinKubernetesVersion: "1.34.0", MinPrometheusVersion: "2.30.0",
			Kubernetes: KubernetesVersionInfo{Version: "v1.34.2", MeetsMinimum: true},
			Nodes:      []NodeVersionInfo{{Name: "n1", KubeletVersion: "v1.20.0", MeetsMinimum: false}},
			NodeCount:  1,
			Prometheus: PrometheusVersionInfo{Error: "unreachable"},
		},
		Metrics: MetricReport{Passed: false, Groups: []MetricGroup{
			{Name: "node-exporter", JobMatcher: `job="node-exporter"`, Required: true, Present: false,
				Checks: []MetricCheck{{Metric: "node_load1", Required: true, Present: false, Error: "no series found in last 15m"}}},
		}},
	}
	resp.Versions.finalize()
	finalizePreflight(resp)
	if resp.Healthy {
		t.Fatal("expected unhealthy")
	}
	// connectivity + node + prometheus version + metric = 4 failures.
	if len(resp.Failures) != 4 {
		t.Fatalf("expected 4 failures, got %d: %+v", len(resp.Failures), resp.Failures)
	}
	steps := map[string]bool{}
	for _, f := range resp.Failures {
		steps[f.Step] = true
	}
	for _, want := range []string{"prometheus_connectivity", "versions", "metrics"} {
		if !steps[want] {
			t.Fatalf("expected a failure for step %q", want)
		}
	}
}
