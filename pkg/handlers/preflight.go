package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/util/version"
)

const (
	// defaultMetricLookback is the window used to detect whether a metric has any
	// series when the caller does not override it.
	defaultMetricLookback = "15m"
	// preflightTimeout bounds the whole preflight so a slow/unreachable Prometheus
	// cannot stall the dashboard's first paint.
	preflightTimeout = 15 * time.Second
	// preflightProbeConcurrency caps how many metric-presence probes run at once.
	preflightProbeConcurrency = 10
	// prometheusUnreachableMsg is the fallback message when Prometheus cannot be
	// reached and no more specific error is available.
	prometheusUnreachableMsg = "Prometheus is not reachable"
)

// Preflight version thresholds are backend policy: the minimum node kubelet and
// Prometheus versions CruiseKube requires. They are fixed here rather than taken
// from the client. Parsed once at init from the shared default constants.
var (
	preflightMinKubeVersion       = version.MustParseGeneric(defaultMinKubeVersion)
	preflightMinKubernetesVersion = version.MustParseGeneric(defaultMinKubernetesVersion)
	preflightMinPrometheusVersion = version.MustParseGeneric(defaultMinPrometheusVersion)
)

// promAPI is the narrow subset of the Prometheus v1.API that the version and
// preflight checks use. Declaring it here (rather than depending on the full
// interface) keeps the checks straightforward to stub in tests. The concrete
// prometheus/client_golang v1.API satisfies it.
type promAPI interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error)
	Buildinfo(ctx context.Context) (promv1.BuildinfoResult, error)
	LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...promv1.Option) ([]string, promv1.Warnings, error)
}

// metricProbe declares a metric CruiseKube relies on, the job matcher that
// scopes it, the scrape-source group it belongs to, and whether its absence
// should block the dashboard. Keep this table in sync with the PromQL the tasks
// actually execute (see pkg/task/utils/prom.go and the metrics provider).
type metricProbe struct {
	Metric     string
	JobMatcher string
	Group      string
	Required   bool
}

// preflightMetricProbes is the declarative list of metrics to verify. Order is
// preserved when grouping for the response. Per decision D3 every probe is
// currently Required; flip Required to demote a metric to non-blocking.
var preflightMetricProbes = []metricProbe{
	// kube-state-metrics — always present on a scraped cluster.
	{"kube_pod_info", `job="kube-state-metrics"`, "kube-state-metrics", true},
	{"kube_pod_status_phase", `job="kube-state-metrics"`, "kube-state-metrics", true},
	{"kube_pod_container_resource_requests", `job="kube-state-metrics"`, "kube-state-metrics", true},
	{"kube_pod_container_status_restarts_total", `job="kube-state-metrics"`, "kube-state-metrics", true},
	{"kube_node_status_allocatable", `job="kube-state-metrics"`, "kube-state-metrics", true},
	{"kube_node_status_capacity", `job="kube-state-metrics"`, "kube-state-metrics", true},
	{"kube_node_labels", `job="kube-state-metrics"`, "kube-state-metrics", true},
	// kube-state-metrics — conditionally present (no taints / no terminations yet
	// is a legitimate healthy state), so probed but NOT required.
	{"kube_node_spec_taint", `job="kube-state-metrics"`, "kube-state-metrics", false},
	{"kube_pod_container_status_last_terminated_reason", `job="kube-state-metrics"`, "kube-state-metrics", false},
	// cAdvisor / kubelet
	{"container_cpu_usage_seconds_total", `job=~"kubelet|kubernetes-nodes-cadvisor"`, "cadvisor-kubelet", true},
	{"container_memory_working_set_bytes", `job=~"kubelet|kubernetes-nodes-cadvisor"`, "cadvisor-kubelet", true},
	// node-exporter
	{"node_load1", `job="node-exporter"`, "node-exporter", true},
	{"node_cpu_seconds_total", `job="node-exporter"`, "node-exporter", true},
	// PSI pressure metrics (Kubernetes 1.34+ requirement — see version checks).
	{"container_pressure_cpu_waiting_seconds_total", `job=~"kubelet|kubernetes-nodes-cadvisor"`, "psi", true},
	{"container_pressure_memory_waiting_seconds_total", `job=~"kubelet|kubernetes-nodes-cadvisor"`, "psi", true},
	{"node_pressure_cpu_waiting_seconds_total", `job="node-exporter"`, "psi", true},
	{"node_pressure_memory_waiting_seconds_total", `job="node-exporter"`, "psi", true},
	// Karpenter — only present on Karpenter clusters and legitimately zero-series
	// when no disruptions have occurred, so probed but NOT required.
	{"karpenter_nodeclaims_disrupted_total", "", "karpenter", false},
}

// PrometheusConnectivity reports whether CruiseKube can reach the configured
// Prometheus and, when it cannot, which target it tried. The bearer token is
// never included.
type PrometheusConnectivity struct {
	Connected bool   `json:"connected"`
	Healthy   bool   `json:"healthy"`
	URL       string `json:"url,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      string `json:"port,omitempty"`
	Probe     string `json:"probe,omitempty"`
	Version   string `json:"version,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Error     string `json:"error,omitempty"`
}

// MetricCheck is the result of probing a single metric for presence. Labels holds
// the distinct label names present on the metric's series (empty for absent
// metrics), so the frontend can show which dimensions each metric exposes.
type MetricCheck struct {
	Metric   string   `json:"metric"`
	Required bool     `json:"required"`
	Present  bool     `json:"present"`
	Series   int      `json:"series"`
	Labels   []string `json:"labels,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// MetricGroup groups metric checks by scrape source.
type MetricGroup struct {
	Name       string        `json:"name"`
	JobMatcher string        `json:"job_matcher,omitempty"`
	Required   bool          `json:"required"`
	Present    bool          `json:"present"`
	Checks     []MetricCheck `json:"checks"`
}

// MetricReport is the metric-existence portion of the preflight response.
type MetricReport struct {
	Passed   bool          `json:"passed"`
	Lookback string        `json:"lookback"`
	Groups   []MetricGroup `json:"groups"`
}

// PreflightFailure is a flat, frontend-renderable description of one thing that
// is wrong, aggregated across every step.
type PreflightFailure struct {
	Step    string `json:"step"`
	Item    string `json:"item"`
	Message string `json:"message"`
}

// PreflightSummary is a headline count of checks.
type PreflightSummary struct {
	TotalChecks int `json:"total_checks"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
}

// PreflightResponse is the payload the dashboard fetches on open.
type PreflightResponse struct {
	ClusterID              string                 `json:"cluster_id"`
	Healthy                bool                   `json:"healthy"`
	GeneratedAt            string                 `json:"generated_at"`
	Summary                PreflightSummary       `json:"summary"`
	Failures               []PreflightFailure     `json:"failures"`
	PrometheusConnectivity PrometheusConnectivity `json:"prometheus_connectivity"`
	Versions               VersionReport          `json:"versions"`
	Metrics                MetricReport           `json:"metrics"`
}

// HandlePreflight runs the dashboard readiness checks: Prometheus connectivity,
// node/Prometheus versions, and the existence of the metrics CruiseKube relies
// on. It always responds 200 with a structured report (unless the request is
// malformed or the cluster is unknown); "setup incomplete" is carried in the
// body via healthy=false and the failures list.
func (deps HandlerDependencies) HandlePreflight(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("cluster.id", clusterID))

	// The version and lookback thresholds are backend policy, not client input.
	// They are fixed constants defined in this package.
	minKubeVer := preflightMinKubeVersion
	minK8sVer := preflightMinKubernetesVersion
	minPromVer := preflightMinPrometheusVersion
	lookback := defaultMetricLookback

	logging.Infof(ctx, "Serving preflight for cluster %s (minKube=%s minK8s=%s minProm=%s lookback=%s) to %s", clusterID, minKubeVer, minK8sVer, minPromVer, lookback, c.ClientIP())

	clients, err := deps.ClusterManager.GetClusterClients(clusterID)
	if err != nil || clients == nil {
		logging.Errorf(ctx, "Failed to get cluster clients for %s: %v", clusterID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("cluster %q not found", clusterID)})
		return
	}

	// Resolve the configured Prometheus target for the connectivity report. A
	// lookup failure is non-fatal — connectivity will simply report an unknown
	// target.
	connInfo, err := deps.ClusterManager.GetPrometheusConnectionInfo(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get Prometheus connection info for %s: %v", clusterID, err)
	}

	ctx, cancel := context.WithTimeout(ctx, preflightTimeout)
	defer cancel()

	resp := PreflightResponse{ClusterID: clusterID, GeneratedAt: time.Now().UTC().Format(time.RFC3339)}

	// Step 1: connectivity & health.
	resp.PrometheusConnectivity = checkPrometheusConnectivity(ctx, clients.PrometheusClient, connInfo)

	// Step 2: versions. Node versions come from the Kubernetes API (independent
	// of Prometheus). The Prometheus version comes from the connectivity probe.
	resp.Versions = buildVersionReport(ctx, clients.KubeClient, minKubeVer, minK8sVer, minPromVer)
	resp.Versions.Prometheus = resolvePrometheusVersion(ctx, clients.PrometheusClient, minPromVer, resp.PrometheusConnectivity)
	resp.Versions.finalize()

	// Step 3: metric existence. Skipped (all failed with the connectivity error)
	// when Prometheus is unreachable.
	connErr := ""
	if !resp.PrometheusConnectivity.Connected {
		connErr = resp.PrometheusConnectivity.Error
		if connErr == "" {
			connErr = prometheusUnreachableMsg
		}
	}
	resp.Metrics = probeMetrics(ctx, clients.PrometheusClient, lookback, connErr)

	finalizePreflight(&resp)

	c.JSON(http.StatusOK, resp)
}

// resolvePrometheusVersion determines the Prometheus version for the version
// report: prefer the buildinfo version from the connectivity probe, fall back to
// the prometheus_build_info metric when reachable but buildinfo was unavailable,
// and otherwise surface the connectivity error.
func resolvePrometheusVersion(ctx context.Context, client promAPI, minVer *version.Version, conn PrometheusConnectivity) PrometheusVersionInfo {
	switch {
	case conn.Version != "":
		return evaluatePrometheusVersion(conn.Version, minVer)
	case conn.Connected:
		return detectPrometheusVersion(ctx, client, minVer)
	default:
		msg := conn.Error
		if msg == "" {
			msg = prometheusUnreachableMsg
		}
		return PrometheusVersionInfo{Error: msg}
	}
}

// checkPrometheusConnectivity verifies CruiseKube can reach Prometheus. It first
// tries the buildinfo API (which also yields the version), then falls back to a
// trivial instant query for backends that don't implement buildinfo. On failure
// the error is prefixed with the target so the user can diagnose it.
func checkPrometheusConnectivity(ctx context.Context, client promAPI, connInfo *cluster.PrometheusConnectionInfo) PrometheusConnectivity {
	conn := PrometheusConnectivity{}
	target := "the configured Prometheus"
	if connInfo != nil && connInfo.URL != "" {
		conn.URL = connInfo.URL
		conn.Host, conn.Port = splitPrometheusTarget(connInfo.URL)
		target = connInfo.URL
	}

	if client == nil {
		conn.Error = "prometheus client is not configured for this cluster"
		return conn
	}

	// Primary probe: buildinfo (also the source of truth for the version).
	if info, err := client.Buildinfo(ctx); err == nil {
		conn.Connected = true
		conn.Healthy = true
		conn.Probe = "buildinfo"
		conn.Version = info.Version
		conn.Revision = info.Revision
		return conn
	} else {
		logging.Infof(ctx, "Prometheus buildinfo probe failed for %s: %v; falling back to instant query", target, err)
	}

	// Fallback probe: a trivial instant query. Distinguishes "unreachable" from
	// "reachable but no buildinfo endpoint" (Thanos/Cortex/Mimir/VictoriaMetrics).
	if _, _, err := client.Query(ctx, "vector(1)", time.Now()); err != nil {
		conn.Error = fmt.Sprintf("failed to reach Prometheus at %s: %v", target, err)
		return conn
	}

	conn.Connected = true
	conn.Healthy = true
	conn.Probe = "query-fallback"
	return conn
}

// splitPrometheusTarget extracts host and port from a Prometheus URL, defaulting
// the port from the scheme when it is not explicit.
func splitPrometheusTarget(rawURL string) (string, string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", ""
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return u.Hostname(), port
}

// probeMetrics checks each metric in preflightMetricProbes for presence. When
// connErr is non-empty (Prometheus unreachable) every check is reported as
// failed with that error instead of running queries.
func probeMetrics(ctx context.Context, client promAPI, lookback, connErr string) MetricReport {
	report := MetricReport{Lookback: lookback, Passed: true}
	checks := make([]MetricCheck, len(preflightMetricProbes))

	if connErr != "" || client == nil {
		if connErr == "" {
			connErr = "prometheus client is not configured for this cluster"
		}
		for i, p := range preflightMetricProbes {
			checks[i] = MetricCheck{Metric: p.Metric, Required: p.Required, Error: connErr}
		}
	} else {
		var wg sync.WaitGroup
		sem := make(chan struct{}, preflightProbeConcurrency)
		for i, p := range preflightMetricProbes {
			wg.Add(1)
			go func(i int, p metricProbe) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				checks[i] = probeOneMetric(ctx, client, p, lookback)
			}(i, p)
		}
		wg.Wait()
	}

	report.Groups = groupMetricChecks(checks)
	for _, g := range report.Groups {
		if !g.Present {
			report.Passed = false
		}
	}
	return report
}

// probeOneMetric runs a presence query for a single metric and, when present,
// fetches the distinct label names on its series.
func probeOneMetric(ctx context.Context, client promAPI, p metricProbe, lookback string) MetricCheck {
	check := MetricCheck{Metric: p.Metric, Required: p.Required}

	selector := p.Metric
	if p.JobMatcher != "" {
		selector = fmt.Sprintf("%s{%s}", p.Metric, p.JobMatcher)
	}

	result, _, err := client.Query(ctx, fmt.Sprintf("count(last_over_time(%s[%s]))", selector, lookback), time.Now())
	if err != nil {
		check.Error = fmt.Sprintf("query failed: %v", err)
		return check
	}
	vec, ok := result.(model.Vector)
	if !ok || len(vec) == 0 {
		check.Error = fmt.Sprintf("no series found in last %s", lookback)
		return check
	}
	check.Series = int(vec[0].Value)
	check.Present = check.Series > 0
	if !check.Present {
		check.Error = fmt.Sprintf("no series found in last %s", lookback)
		return check
	}

	// Present: enumerate the distinct label names on this metric. A label lookup
	// failure is non-fatal — the metric is still present.
	labels, lerr := metricLabelNames(ctx, client, selector, lookback)
	if lerr != nil {
		logging.Infof(ctx, "Failed to fetch label names for metric %s: %v", p.Metric, lerr)
	} else {
		check.Labels = labels
	}
	return check
}

// metricLabelNames returns the distinct label names present on the series matched
// by selector over the lookback window, excluding the internal __name__ label.
func metricLabelNames(ctx context.Context, client promAPI, selector, lookback string) ([]string, error) {
	dur, err := time.ParseDuration(lookback)
	if err != nil {
		dur = 15 * time.Minute
	}
	end := time.Now()
	names, _, err := client.LabelNames(ctx, []string{selector}, end.Add(-dur), end)
	if err != nil {
		return nil, fmt.Errorf("label names query failed: %w", err)
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n == "__name__" {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// groupMetricChecks folds per-metric checks into ordered scrape-source groups.
// A group is present iff all of its required checks are present.
func groupMetricChecks(checks []MetricCheck) []MetricGroup {
	groups := make([]MetricGroup, 0)
	index := make(map[string]int)
	for i, p := range preflightMetricProbes {
		gi, ok := index[p.Group]
		if !ok {
			gi = len(groups)
			index[p.Group] = gi
			groups = append(groups, MetricGroup{Name: p.Group, JobMatcher: p.JobMatcher, Present: true})
		}
		g := &groups[gi]
		if p.Required {
			g.Required = true
			if !checks[i].Present {
				g.Present = false
			}
		}
		g.Checks = append(g.Checks, checks[i])
	}
	return groups
}

// finalizePreflight computes the overall health, the summary counts, and the
// flat failures list from the already-populated step results.
func finalizePreflight(resp *PreflightResponse) {
	failures := make([]PreflightFailure, 0)
	total, passed := 0, 0

	// Connectivity counts as a single check.
	total++
	if resp.PrometheusConnectivity.Connected && resp.PrometheusConnectivity.Healthy {
		passed++
	} else {
		msg := resp.PrometheusConnectivity.Error
		if msg == "" {
			msg = prometheusUnreachableMsg
		}
		failures = append(failures, PreflightFailure{Step: "prometheus_connectivity", Item: "prometheus", Message: msg})
	}

	// Kubernetes server (control-plane) version check.
	total++
	if resp.Versions.Kubernetes.MeetsMinimum {
		passed++
	} else {
		msg := resp.Versions.Kubernetes.Error
		if msg == "" {
			msg = fmt.Sprintf("kubernetes server version %s is below minimum %s", resp.Versions.Kubernetes.Version, resp.Versions.MinKubernetesVersion)
		}
		failures = append(failures, PreflightFailure{Step: "versions", Item: "kubernetes", Message: msg})
	}

	// Node version checks (one per node), plus a node-listing failure if any.
	for _, n := range resp.Versions.Nodes {
		total++
		if n.MeetsMinimum {
			passed++
			continue
		}
		msg := n.Error
		if msg == "" {
			msg = fmt.Sprintf("kubelet version %s is below minimum %s", n.KubeletVersion, resp.Versions.MinKubeVersion)
		}
		failures = append(failures, PreflightFailure{Step: "versions", Item: n.Name, Message: msg})
	}
	if resp.Versions.NodeError != "" {
		total++
		failures = append(failures, PreflightFailure{Step: "versions", Item: "nodes", Message: resp.Versions.NodeError})
	}

	// Prometheus version check.
	total++
	if resp.Versions.Prometheus.MeetsMinimum {
		passed++
	} else {
		msg := resp.Versions.Prometheus.Error
		if msg == "" {
			msg = fmt.Sprintf("prometheus version %s is below minimum %s", resp.Versions.Prometheus.Version, resp.Versions.MinPrometheusVersion)
		}
		failures = append(failures, PreflightFailure{Step: "versions", Item: "prometheus", Message: msg})
	}

	// Metric checks.
	for _, g := range resp.Metrics.Groups {
		matcher := ""
		if g.JobMatcher != "" {
			matcher = "{" + g.JobMatcher + "}"
		}
		for _, ck := range g.Checks {
			total++
			if ck.Present {
				passed++
				continue
			}
			detail := ck.Error
			if detail == "" {
				detail = "metric not found"
			}
			failures = append(failures, PreflightFailure{
				Step:    "metrics",
				Item:    ck.Metric,
				Message: fmt.Sprintf("metric %s%s: %s", ck.Metric, matcher, detail),
			})
		}
	}

	resp.Summary = PreflightSummary{TotalChecks: total, Passed: passed, Failed: total - passed}
	resp.Failures = failures
	resp.Healthy = resp.PrometheusConnectivity.Connected &&
		resp.PrometheusConnectivity.Healthy &&
		resp.Versions.Passed &&
		resp.Metrics.Passed
}
