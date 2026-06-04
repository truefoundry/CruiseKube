#!/usr/bin/env python3
"""
Validate that a Prometheus instance (port-forwarded locally) exposes metrics
required by CruiseKube, and report cluster scrape-source configuration.

Usage:
  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
  python3 scripts/check_prometheus_metrics.py --port 9090

Uses the current kubectl context unless --context is set.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Sequence, Tuple


# --- CruiseKube metric catalog (aligned with docs/documentation/operate/operate-troubleshooting.md) ---

@dataclass(frozen=True)
class MetricCheck:
    name: str
    source: str
    query: str
    purpose: str
    optional: bool = False


METRIC_CHECKS: Sequence[MetricCheck] = (
    # kubelet / cAdvisor (job is kubelet or kubernetes-nodes-cadvisor in kube-prometheus-stack)
    MetricCheck(
        "container_cpu_usage_seconds_total",
        "kubelet",
        'count(container_cpu_usage_seconds_total{job=~"kubelet|kubernetes-nodes-cadvisor"})',
        "Workload CPU usage and cluster CPU utilization",
    ),
    MetricCheck(
        "container_memory_working_set_bytes",
        "kubelet",
        'count(container_memory_working_set_bytes{job=~"kubelet|kubernetes-nodes-cadvisor"})',
        "Workload memory usage and cluster memory utilization",
    ),
    MetricCheck(
        "container_pressure_cpu_waiting_seconds_total",
        "kubelet",
        'count(container_pressure_cpu_waiting_seconds_total{job=~"kubelet|kubernetes-nodes-cadvisor"})',
        "PSI-aware CPU signals (Kubernetes 1.34+ / kernel PSI)",
    ),
    MetricCheck(
        "container_pressure_memory_waiting_seconds_total",
        "kubelet",
        'count(container_pressure_memory_waiting_seconds_total{job=~"kubelet|kubernetes-nodes-cadvisor"})',
        "Container memory pressure (PSI)",
    ),
    # kube-state-metrics
    MetricCheck(
        "kube_pod_info",
        "kube-state-metrics",
        'count(kube_pod_info{job="kube-state-metrics"})',
        "Map pods to owning workloads",
    ),
    MetricCheck(
        "kube_pod_status_phase",
        "kube-state-metrics",
        'count(kube_pod_status_phase{job="kube-state-metrics"})',
        "Running/Pending pods and workload discovery",
    ),
    MetricCheck(
        "kube_pod_container_resource_requests (cpu)",
        "kube-state-metrics",
        'count(kube_pod_container_resource_requests{job="kube-state-metrics",resource="cpu"})',
        "CPU requests for recommendations",
    ),
    MetricCheck(
        "kube_pod_container_resource_requests (memory)",
        "kube-state-metrics",
        'count(kube_pod_container_resource_requests{job="kube-state-metrics",resource="memory"})',
        "Memory requests for recommendations",
    ),
    MetricCheck(
        "kube_node_status_allocatable",
        "kube-state-metrics",
        'count(kube_node_status_allocatable{job="kube-state-metrics"})',
        "Node allocatable CPU/memory capacity",
    ),
    MetricCheck(
        "kube_node_status_capacity",
        "kube-state-metrics",
        'count(kube_node_status_capacity{job="kube-state-metrics",resource="cpu"})',
        "Node CPU capacity (load ratio)",
    ),
    MetricCheck(
        "kube_pod_container_status_last_terminated_reason",
        "kube-state-metrics",
        'count(kube_pod_container_status_last_terminated_reason{job="kube-state-metrics"})',
        "OOM detection (reason=OOMKilled)",
    ),
    MetricCheck(
        "kube_pod_container_status_restarts_total",
        "kube-state-metrics",
        'count(kube_pod_container_status_restarts_total{job="kube-state-metrics"})',
        "OOM / restart correlation",
    ),
    MetricCheck(
        "kube_node_spec_taint",
        "kube-state-metrics",
        'count(kube_node_spec_taint{job="kube-state-metrics"})',
        "Exclude GPU nodes from cluster rollups",
    ),
    MetricCheck(
        "kube_node_labels",
        "kube-state-metrics",
        'count(kube_node_labels{job="kube-state-metrics"})',
        "Exclude NVIDIA accelerator nodes from rollups",
    ),
    # node-exporter
    MetricCheck(
        "node_cpu_seconds_total",
        "node-exporter",
        'count(node_cpu_seconds_total{job="node-exporter"})',
        "Cluster CPU utilization rollups",
    ),
    MetricCheck(
        "node_load1",
        "node-exporter",
        'count(node_load1{job="node-exporter"})',
        "Node load monitoring",
    ),
    MetricCheck(
        "node_pressure_cpu_waiting_seconds_total",
        "node-exporter",
        'count(node_pressure_cpu_waiting_seconds_total{job="node-exporter"})',
        "Node CPU pressure (PSI)",
    ),
    MetricCheck(
        "node_pressure_memory_waiting_seconds_total",
        "node-exporter",
        'count(node_pressure_memory_waiting_seconds_total{job="node-exporter"})',
        "Node memory pressure (PSI)",
    ),
    MetricCheck(
        "node_memory_MemTotal_bytes",
        "node-exporter",
        'count(node_memory_MemTotal_bytes{job="node-exporter"})',
        "Cluster memory utilization",
    ),
    MetricCheck(
        "node_memory_MemFree_bytes",
        "node-exporter",
        'count(node_memory_MemFree_bytes{job="node-exporter"})',
        "Cluster memory utilization",
    ),
    MetricCheck(
        "node_memory_Buffers_bytes",
        "node-exporter",
        'count(node_memory_Buffers_bytes{job="node-exporter"})',
        "Cluster memory utilization",
    ),
    MetricCheck(
        "node_memory_Cached_bytes",
        "node-exporter",
        'count(node_memory_Cached_bytes{job="node-exporter"})',
        "Cluster memory utilization",
    ),
    # optional
    MetricCheck(
        "karpenter_nodeclaims_disrupted_total",
        "karpenter",
        "count(karpenter_nodeclaims_disrupted_total)",
        "Karpenter consolidation/eviction export (only if Karpenter is installed)",
        optional=True,
    ),
)

SOURCE_DOCS: Dict[str, str] = {
    "kubelet": (
        "kubelet / cAdvisor on each node (container_* metrics). "
        "kube-prometheus-stack usually exposes these via scrape pools "
        "kubernetes-nodes-cadvisor (cAdvisor) and/or kubelet — CruiseKube accepts either job label."
    ),
    "kube-state-metrics": (
        "kube-state-metrics Deployment (kube_* metrics). "
        "Expose port 8080; scrape via ServiceMonitor or endpoints discovery."
    ),
    "node-exporter": (
        "node-exporter DaemonSet on each node (node_* metrics), host port 9100. "
        "Only one DaemonSet per node; scrape via ServiceMonitor or endpoints discovery."
    ),
    "karpenter": (
        "Karpenter controller metrics endpoint (only when Karpenter is installed)."
    ),
}

SOURCE_EXPECTED_JOBS: Dict[str, frozenset[str]] = {
    "kubelet": frozenset({"kubelet", "kubernetes-nodes-cadvisor"}),
    "kube-state-metrics": frozenset({"kube-state-metrics"}),
    "node-exporter": frozenset({"node-exporter"}),
}

# Human-readable summary for reports.
SOURCE_JOB: Dict[str, str] = {
    "kubelet": 'kubelet or kubernetes-nodes-cadvisor',
    "kube-state-metrics": "kube-state-metrics",
    "node-exporter": "node-exporter",
}

# Scrape pool / job name patterns (kube-prometheus-stack and standalone chart variants).
SOURCE_SCRAPE_POOL_PATTERNS: Dict[str, re.Pattern[str]] = {
    "kubelet": re.compile(r"kubelet|kubernetes-nodes-cadvisor|cadvisor", re.I),
    "kube-state-metrics": re.compile(r"kube[-_]state[-_]metrics", re.I),
    "node-exporter": re.compile(r"node[-_]exporter|prometheus-node-exporter", re.I),
}


@dataclass
class ScrapePoolSummary:
    name: str
    active: int = 0
    up: int = 0
    down: int = 0
    unknown: int = 0
    dropped: int = 0
    sample_addresses: List[str] = field(default_factory=list)


@dataclass
class ServiceDiscoveryReport:
    scrape_pools: List[str]
    pool_summaries: Dict[str, ScrapePoolSummary]
    pools_for_source: Dict[str, List[str]]
    ui_service_discovery: str
    ui_targets: str
    ui_service_discovery_by_source: Dict[str, str]


@dataclass
class MetricResult:
    check: MetricCheck
    ok: bool
    series_count: int = 0
    alt_jobs: List[str] = field(default_factory=list)
    error: Optional[str] = None
    # False when the metric exists but not with the job label CruiseKube uses in PromQL.
    expected_job_ok: bool = True


@dataclass
class ClusterSourceInfo:
    source: str
    configured: bool
    summary: str
    details: List[str] = field(default_factory=list)


# --- Terminal styling ---

def _supports_color() -> bool:
    return sys.stdout.isatty() and not bool(
        __import__("os").environ.get("NO_COLOR")
    )


def _c(code: str, text: str) -> str:
    if not _supports_color():
        return text
    return f"\033[{code}m{text}\033[0m"


def green(t: str) -> str:
    return _c("32", t)


def red(t: str) -> str:
    return _c("31;1", t)


def yellow(t: str) -> str:
    return _c("33", t)


def bold(t: str) -> str:
    return _c("1", t)


def log_progress(message: str, *, quiet: bool = False) -> None:
    """Progress lines go to stderr so they stay separate from the final report."""
    if quiet:
        return
    ts = time.strftime("%H:%M:%S")
    print(f"[{ts}] {message}", file=sys.stderr, flush=True)


# --- Prometheus HTTP ---

class PrometheusClient:
    def __init__(self, base_url: str, timeout: float = 30.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def _get(self, path: str, params: Optional[Dict[str, str]] = None) -> Dict[str, Any]:
        url = f"{self.base_url}{path}"
        if params:
            url += "?" + urllib.parse.urlencode(params)
        req = urllib.request.Request(url, headers={"Accept": "application/json"})
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                body = resp.read().decode("utf-8")
        except urllib.error.URLError as exc:
            raise RuntimeError(f"Cannot reach Prometheus at {self.base_url}: {exc}") from exc
        data = json.loads(body)
        if data.get("status") != "success":
            raise RuntimeError(
                f"Prometheus API error for {path}: {data.get('error', data)}"
            )
        return data

    def query_scalar_count(self, promql: str) -> Tuple[int, Optional[str]]:
        data = self._get("/api/v1/query", {"query": promql})
        result = data.get("data", {}).get("result", [])
        if not result:
            return 0, None
        item = result[0]
        if item.get("value"):
            try:
                return int(float(item["value"][1])), None
            except (TypeError, ValueError, IndexError) as exc:
                return 0, str(exc)
        # vector or matrix with samples — treat non-empty as present
        if item.get("metric") or item.get("values"):
            return 1, None
        return 0, None

    def jobs_for_metric(self, metric_name: str) -> List[str]:
        base = _metric_base_name(metric_name)
        promql = f"count by (job) ({base})"
        data = self._get("/api/v1/query", {"query": promql})
        jobs: List[str] = []
        for item in data.get("data", {}).get("result", []):
            job = item.get("metric", {}).get("job")
            if job:
                jobs.append(job)
        return sorted(set(jobs))

    def targets_state(self) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
        data = self._get("/api/v1/targets", {"state": "any"})
        payload = data.get("data", {})
        return (
            payload.get("activeTargets", []) or [],
            payload.get("droppedTargets", []) or [],
        )

    def scrape_pools(self) -> List[str]:
        try:
            data = self._get("/api/v1/scrape_pools")
        except RuntimeError:
            return []
        pools = data.get("data", {}).get("scrapePools", [])
        return sorted(pools) if isinstance(pools, list) else []

    def targets_for_pool(self, pool: str) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
        data = self._get("/api/v1/targets", {"scrapePool": pool, "state": "any"})
        payload = data.get("data", {})
        return (
            payload.get("activeTargets", []) or [],
            payload.get("droppedTargets", []) or [],
        )

    @property
    def ui_service_discovery(self) -> str:
        return f"{self.base_url}/service-discovery"

    @property
    def ui_targets(self) -> str:
        return f"{self.base_url}/targets"


def _target_address(target: Dict[str, Any]) -> str:
    labels = target.get("discoveredLabels") or target.get("labels") or {}
    return str(labels.get("__address__", labels.get("instance", "?")))


def _summarize_pool(
    name: str,
    active: List[Dict[str, Any]],
    dropped: List[Dict[str, Any]],
) -> ScrapePoolSummary:
    summary = ScrapePoolSummary(name=name, active=len(active), dropped=len(dropped))
    for t in active:
        health = (t.get("health") or "unknown").lower()
        if health == "up":
            summary.up += 1
        elif health == "down":
            summary.down += 1
        else:
            summary.unknown += 1
        if len(summary.sample_addresses) < 3:
            summary.sample_addresses.append(_target_address(t))
    return summary


def build_service_discovery_report(
    client: PrometheusClient,
    active_targets: List[Dict[str, Any]],
    dropped_targets: List[Dict[str, Any]],
    *,
    quiet: bool,
) -> ServiceDiscoveryReport:
    """Mirror the Prometheus /service-discovery and /targets UIs via the HTTP API."""
    log_progress("Fetching scrape pools (same data as /service-discovery UI)...", quiet=quiet)
    pool_names = client.scrape_pools()

    by_pool: Dict[str, List[Dict[str, Any]]] = {}
    dropped_by_pool: Dict[str, List[Dict[str, Any]]] = {}
    for t in active_targets:
        pool = t.get("scrapePool") or (t.get("labels") or {}).get("job", "unknown")
        by_pool.setdefault(pool, []).append(t)
    for t in dropped_targets:
        pool = t.get("scrapePool") or (t.get("labels") or {}).get("job", "unknown")
        dropped_by_pool.setdefault(pool, []).append(t)

    if not pool_names:
        pool_names = sorted(set(by_pool) | set(dropped_by_pool))
        log_progress(
            "  scrape_pools API unavailable; derived pool names from /api/v1/targets",
            quiet=quiet,
        )
    else:
        log_progress(f"  found {len(pool_names)} scrape pool(s): {', '.join(pool_names)}", quiet=quiet)

    summaries: Dict[str, ScrapePoolSummary] = {}
    for pool in pool_names:
        active = by_pool.get(pool, [])
        dropped = dropped_by_pool.get(pool, [])
        if not active and not dropped:
            log_progress(f"  loading targets for scrape pool {pool!r}...", quiet=quiet)
            active, dropped = client.targets_for_pool(pool)
        summaries[pool] = _summarize_pool(pool, active, dropped)

    pools_for_source: Dict[str, List[str]] = {}
    ui_by_source: Dict[str, str] = {}
    for source, pattern in SOURCE_SCRAPE_POOL_PATTERNS.items():
        matched = [p for p in pool_names if pattern.search(p)]
        if not matched:
            matched = [p for p in summaries if pattern.search(p)]
        pools_for_source[source] = matched
        if matched:
            ui_by_source[source] = (
                f"{client.ui_service_discovery}?search={urllib.parse.quote(matched[0])}"
            )
        else:
            ui_by_source[source] = client.ui_service_discovery

    return ServiceDiscoveryReport(
        scrape_pools=pool_names,
        pool_summaries=summaries,
        pools_for_source=pools_for_source,
        ui_service_discovery=client.ui_service_discovery,
        ui_targets=client.ui_targets,
        ui_service_discovery_by_source=ui_by_source,
    )


# --- kubectl cluster inspection ---

def kubectl_json(args: List[str], context: str) -> Any:
    cmd = ["kubectl", "--context", context] + args + ["-o", "json"]
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except subprocess.CalledProcessError as exc:
        stderr = (exc.stderr or "").strip()
        raise RuntimeError(f"kubectl failed ({' '.join(cmd)}): {stderr}") from exc
    return json.loads(out.stdout)


def current_context() -> str:
    out = subprocess.run(
        ["kubectl", "config", "current-context"],
        capture_output=True,
        text=True,
        check=True,
    )
    return out.stdout.strip()


def list_resources(
    context: str, resource: str, label_selector: Optional[str] = None
) -> List[Dict[str, Any]]:
    args = ["get", resource, "-A"]
    if label_selector:
        args.extend(["-l", label_selector])
    try:
        data = kubectl_json(args, context)
    except RuntimeError:
        return []
    return data.get("items", [])


def inspect_cluster_sources(context: str) -> Dict[str, ClusterSourceInfo]:
    sources: Dict[str, ClusterSourceInfo] = {}

    # kube-state-metrics workloads
    ksm_items: List[str] = []
    for kind in ("deploy", "sts"):
        for item in list_resources(context, kind):
            name = item["metadata"]["name"]
            ns = item["metadata"]["namespace"]
            if "kube-state-metrics" in name:
                ready = item.get("status", {}).get("readyReplicas") or item.get(
                    "status", {}
                ).get("readyReplicas")
                replicas = item.get("spec", {}).get("replicas", "?")
                ksm_items.append(f"{kind}/{ns}/{name} (ready={ready}/{replicas})")
    ksm_svcs = [
        f"svc/{i['metadata']['namespace']}/{i['metadata']['name']}"
        for i in list_resources(context, "svc")
        if "kube-state-metrics" in i["metadata"]["name"]
    ]
    ksm_monitors = _find_servicemonitors(context, r"kube-state-metrics|kube_state_metrics")
    sources["kube-state-metrics"] = ClusterSourceInfo(
        source="kube-state-metrics",
        configured=bool(ksm_items or ksm_svcs),
        summary=(
            f"{len(ksm_items)} workload(s), {len(ksm_svcs)} Service(s), "
            f"{len(ksm_monitors)} ServiceMonitor(s)"
        ),
        details=ksm_items + ksm_svcs + ksm_monitors,
    )

    # node-exporter
    ne_items: List[str] = []
    for item in list_resources(context, "ds"):
        name = item["metadata"]["name"]
        if "node-exporter" in name or "node_exporter" in name:
            ns = item["metadata"]["namespace"]
            desired = item.get("status", {}).get("desiredNumberScheduled", "?")
            ready = item.get("status", {}).get("numberReady", "?")
            ne_items.append(f"ds/{ns}/{name} (ready={ready}/{desired})")
    ne_monitors = _find_servicemonitors(
        context, r"node-exporter|node_exporter|prometheus-node-exporter"
    )
    sources["node-exporter"] = ClusterSourceInfo(
        source="node-exporter",
        configured=bool(ne_items),
        summary=f"{len(ne_items)} DaemonSet(s), {len(ne_monitors)} ServiceMonitor(s)",
        details=ne_items + ne_monitors,
    )

    # kubelet scrape (ServiceMonitor / PodMonitor / operator endpoints)
    kubelet_monitors = _find_servicemonitors(context, r"kubelet|cadvisor|cAdvisor")
    kubelet_eps = [
        f"ep/{i['metadata']['namespace']}/{i['metadata']['name']}"
        for i in list_resources(context, "ep")
        if i["metadata"]["name"] in ("kubelet", "prometheus-kube-prometheus-kubelet")
        or "kubelet" in i["metadata"]["name"]
    ]
    sources["kubelet"] = ClusterSourceInfo(
        source="kubelet",
        configured=bool(kubelet_monitors or kubelet_eps),
        summary=(
            f"{len(kubelet_monitors)} ServiceMonitor(s)/PodMonitor(s), "
            f"{len(kubelet_eps)} kubelet Endpoint(s)"
        ),
        details=kubelet_monitors + kubelet_eps,
    )

    return sources


def _find_servicemonitors(context: str, pattern: str) -> List[str]:
    found: List[str] = []
    regex = re.compile(pattern, re.I)
    for crd in ("servicemonitor", "podmonitor"):
        try:
            items = list_resources(context, crd)
        except RuntimeError:
            continue
        for item in items:
            name = item["metadata"]["name"]
            ns = item["metadata"]["namespace"]
            if regex.search(name):
                found.append(f"{crd}/{ns}/{name}")
                continue
            labels = item["metadata"].get("labels") or {}
            if any(regex.search(str(v)) for v in labels.values()):
                found.append(f"{crd}/{ns}/{name}")
    return found


def scrape_targets_for_source(
    targets: List[Dict[str, Any]], source: str
) -> List[str]:
    """Summarize Prometheus scrape targets related to a metric source."""
    keywords = {
        "kubelet": ("kubelet", "kubernetes-nodes-cadvisor", "cadvisor", "cAdvisor"),
        "kube-state-metrics": ("kube-state-metrics", "kube_state_metrics"),
        "node-exporter": ("node-exporter", "node_exporter", "prometheus-node-exporter"),
        "karpenter": ("karpenter",),
    }
    keys = keywords.get(source, ())
    lines: List[str] = []
    for t in targets:
        labels = t.get("labels") or {}
        scrape_url = t.get("scrapeUrl", "")
        job = labels.get("job", "")
        health = t.get("health", "unknown")
        blob = f"{job} {scrape_url} {json.dumps(labels)}"
        if not any(k.lower() in blob.lower() for k in keys):
            continue
        lines.append(
            f"job={job} health={health} url={scrape_url.split('?')[0]}"
        )
    return lines[:12]


# --- Main checks ---

def _metric_base_name(name: str) -> str:
    return name.split(" ", 1)[0].split(" (", 1)[0]


def _strip_job_selector(promql: str) -> str:
    """Remove job=\"...\" from a count(<selector>) query, keeping other label matchers."""

    def repl(match: re.Match[str]) -> str:
        prefix = match.group(1)
        selector = match.group(2)
        inner = selector[1:-1]
        inner = re.sub(r',?\s*job\s*=\s*"[^"]*"', "", inner)
        inner = re.sub(r'job\s*=\s*"[^"]*"\s*,?\s*', "", inner)
        inner = re.sub(r',?\s*job\s*=~\s*"[^"]*"', "", inner)
        inner = re.sub(r'job\s*=~\s*"[^"]*"\s*,?\s*', "", inner)
        inner = inner.strip().strip(",").strip()
        if inner:
            return f"count({prefix}{{{inner}}})"
        return f"count({prefix})"

    return re.sub(
        r"count\(([\w:]+)(\{[^}]+\})\)",
        repl,
        promql,
        count=1,
    )


def _fallback_queries(strict_query: str, metric_name: str) -> List[str]:
    """Queries to run when the strict (job-labeled) check returns no series."""
    fallbacks: List[str] = []
    if 'job="' in strict_query or 'job=~"' in strict_query:
        stripped = _strip_job_selector(strict_query)
        if stripped != strict_query:
            fallbacks.append(stripped)
    fallbacks.append(f"count({metric_name})")
    # Deduplicate while preserving order.
    seen: set[str] = set()
    ordered: List[str] = []
    for q in fallbacks:
        if q not in seen:
            seen.add(q)
            ordered.append(q)
    return ordered


def evaluate_metric(client: PrometheusClient, check: MetricCheck) -> MetricResult:
    """
    Pass if the metric exists in this Prometheus under any job label.
    Warn (still pass) when only non-standard job labels are present.
    """
    count, err = client.query_scalar_count(check.query)
    if count > 0:
        return MetricResult(
            check=check,
            ok=True,
            series_count=count,
            expected_job_ok=True,
            error=err,
        )

    metric = _metric_base_name(check.name)
    for fallback in _fallback_queries(check.query, metric):
        count, err = client.query_scalar_count(fallback)
        if count > 0:
            alt_jobs = client.jobs_for_metric(metric)
            expected = SOURCE_EXPECTED_JOBS.get(check.source, frozenset())
            job_ok = not expected or bool(set(alt_jobs) & expected)
            return MetricResult(
                check=check,
                ok=True,
                series_count=count,
                alt_jobs=alt_jobs,
                expected_job_ok=job_ok,
                error=err,
            )

    alt_jobs = client.jobs_for_metric(metric)
    return MetricResult(
        check=check,
        ok=False,
        alt_jobs=alt_jobs,
        expected_job_ok=False,
        error=err,
    )


def run_metric_checks(
    client: PrometheusClient,
    include_optional: bool,
    *,
    quiet: bool,
) -> List[MetricResult]:
    checks = [
        c for c in METRIC_CHECKS if include_optional or not c.optional
    ]
    total = len(checks)
    results: List[MetricResult] = []
    log_progress(f"Checking {total} metric(s) via PromQL...", quiet=quiet)
    for idx, check in enumerate(checks, start=1):
        log_progress(f"  [{idx}/{total}] {check.name}", quiet=quiet)
        try:
            result = evaluate_metric(client, check)
            results.append(result)
            if not result.ok:
                status = "MISSING"
            elif not result.expected_job_ok:
                jobs = result.alt_jobs or ["?"]
                status = f"ok (non-standard job: {', '.join(jobs)})"
            else:
                status = "ok"
            log_progress(
                f"       -> {status} (series={result.series_count})",
                quiet=quiet,
            )
        except RuntimeError as exc:
            results.append(
                MetricResult(check=check, ok=False, error=str(exc))
            )
            log_progress(f"       -> error: {exc}", quiet=quiet)
    return results


def print_service_discovery_section(
    sd: ServiceDiscoveryReport,
    failed_sources: List[str],
) -> None:
    print(bold("Service discovery & scrape targets (Prometheus UI)"))
    print("-" * 72)
    print(f"  Open in browser: {sd.ui_service_discovery}")
    print(f"  Active targets:  {sd.ui_targets}")
    print("  (Same information as above is loaded from /api/v1/scrape_pools and /api/v1/targets.)")
    print()

    for source in ("kube-state-metrics", "node-exporter", "kubelet"):
        pools = sd.pools_for_source.get(source, [])
        ui = sd.ui_service_discovery_by_source.get(source, sd.ui_service_discovery)
        print(f"  {bold(source)}")
        print(f"    UI filter: {ui}")
        if not pools:
            print(red("    No matching scrape pool configured in this Prometheus."))
            print(
                yellow(
                    f"    Expected a scrape pool matching job={SOURCE_JOB.get(source)!r} "
                    "(see prometheus.yml or ServiceMonitors)."
                )
            )
            continue
        for pool in pools:
            summary = sd.pool_summaries.get(pool)
            if not summary:
                print(f"    pool {pool!r}: (no targets loaded)")
                continue
            health = f"up={summary.up} down={summary.down} unknown={summary.unknown}"
            dropped = (
                f", dropped={summary.dropped}" if summary.dropped else ""
            )
            mark = green("✓") if summary.up > 0 else red("✗")
            print(
                f"    {mark} scrape pool {pool!r}: "
                f"{summary.active} active target(s) ({health}{dropped})"
            )
            if summary.sample_addresses:
                print(
                    "         sample __address__: "
                    + ", ".join(summary.sample_addresses)
                )
            if summary.active > 0 and summary.up == 0:
                print(
                    yellow(
                        "         All active targets are down — open /targets for last errors."
                    )
                )
        print()

    if failed_sources:
        print(yellow("  Tip: For missing metrics, compare the failing source above with"))
        print(yellow(f"       {sd.ui_service_discovery} and fix scrape config / relabel rules."))
        print()


def print_report(
    prom_url: str,
    context: str,
    results: List[MetricResult],
    cluster_sources: Dict[str, ClusterSourceInfo],
    targets: List[Dict[str, Any]],
    sd_report: ServiceDiscoveryReport,
    include_optional: bool,
) -> int:
    required = [r for r in results if not r.check.optional]
    optional = [r for r in results if r.check.optional]
    failed = [r for r in required if not r.ok]
    warned = [r for r in required if r.ok and not r.expected_job_ok]
    passed = [r for r in required if r.ok]

    print()
    print(bold("CruiseKube Prometheus metrics check"))
    print(f"  Prometheus: {prom_url}")
    print(f"  Cluster:    {bold(context)} (kubectl current context)")
    print()

    print(bold(f"Results: {len(passed)}/{len(required)} required metrics present"))
    if warned:
        print(
            yellow(
                f"         {len(warned)} present with non-standard job labels "
                f"(CruiseKube PromQL expects {', '.join(SOURCE_JOB.values())})"
            )
        )
    if include_optional and optional:
        opt_ok = sum(1 for r in optional if r.ok)
        print(f"         {opt_ok}/{len(optional)} optional metrics present")
    print()

    failed_sources = sorted({r.check.source for r in failed})
    print_service_discovery_section(sd_report, failed_sources)

    if warned:
        print(yellow(bold("METRICS PRESENT (non-standard job labels)")))
        print("-" * 72)
        for r in warned:
            chk = r.check
            expected_job = SOURCE_JOB.get(chk.source)
            print(yellow(f"  ! {chk.name}"))
            print(f"      Source:   {bold(chk.source)}")
            print(f"      Found:    job label(s) {r.alt_jobs!r}")
            print(
                f"      Expected: job={expected_job!r} for CruiseKube queries "
                f"({chk.query})"
            )
            print(
                yellow(
                    "      Note:     Metric data exists; relabel scrape config or "
                    "update CruiseKube only if queries still fail."
                )
            )
            print()

    if failed:
        print(red(bold("MISSING METRICS")))
        print("-" * 72)
        for r in failed:
            chk = r.check
            print(red(f"  ✗ {chk.name}"))
            print(f"      Source:   {bold(chk.source)} — {SOURCE_DOCS.get(chk.source, '')}")
            print(f"      Purpose:  {chk.purpose}")
            print(f"      PromQL:   {chk.query}")
            if r.series_count == 0 and not r.error:
                print(
                    yellow(
                        "      Note:     no time series found (not scraped, filtered, "
                        "or wrong Prometheus instance)"
                    )
                )
            if r.error:
                print(f"      Error:    {r.error}")

            src_info = cluster_sources.get(chk.source)
            if src_info:
                status = green("configured in cluster") if src_info.configured else red(
                    "not detected in cluster"
                )
                print(f"      Cluster:  {status} — {src_info.summary}")
                for line in src_info.details[:6]:
                    print(f"                - {line}")
                if not src_info.configured:
                    print(
                        yellow(
                            "                Install or expose the exporter, then add a "
                            "scrape config / ServiceMonitor to this Prometheus."
                        )
                    )

            target_lines = scrape_targets_for_source(targets, chk.source)
            sd_pools = sd_report.pools_for_source.get(chk.source, [])
            if sd_pools:
                print(
                    f"      Discovery: scrape pool(s) {sd_pools!r} — "
                    f"{sd_report.ui_service_discovery_by_source.get(chk.source, sd_report.ui_service_discovery)}"
                )
            if target_lines:
                print("      Scrapes:  (active targets in this Prometheus)")
                for line in target_lines[:5]:
                    print(f"                - {line}")
            elif src_info and src_info.configured:
                print(
                    yellow(
                        "      Scrapes:  no matching targets in this Prometheus — "
                        "exporter may exist but not be scraped here"
                    )
                )
            print()
    elif not warned:
        print(green(bold("All required metrics are present.")))
        print()

    # Brief pass list when there are failures
    if passed and failed:
        print(bold("Present metrics:"))
        for r in passed:
            line = f"  ✓ {r.check.name} ({r.series_count} series)"
            if not r.expected_job_ok:
                line += f" (job {r.alt_jobs})"
                print(yellow(line))
            else:
                print(green(line))
        print()

    # Cluster source overview
    print(bold("Cluster scrape sources (kubectl)"))
    print("-" * 72)
    for key in ("kube-state-metrics", "node-exporter", "kubelet"):
        info = cluster_sources[key]
        mark = green("✓") if info.configured else red("✗")
        print(f"  {mark} {info.source}: {info.summary}")
        for line in info.details[:4]:
            print(f"       {line}")
        if not info.details:
            print(yellow("       No matching workloads or ServiceMonitors found."))
    print()

    if failed:
        print(bold("How to fix missing metrics"))
        print("  1. Port-forward the Prometheus CruiseKube will query (this script tests that instance).")
        print("  2. Ensure kube-state-metrics, node-exporter, and kubelet/cAdvisor are scraped into it.")
        print("  3. Use job labels kube-state-metrics, node-exporter, and kubelet (kube-prometheus-stack defaults).")
        print("  4. See docs/install/gs-prerequisites.md#scenario-3-dedicated-standalone-prometheus")
        print("     if production Prometheus filters or aggregates metrics away.")
        print()

    return 1 if failed else 0


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description=(
            "Check CruiseKube-required metrics in a port-forwarded Prometheus "
            "and summarize cluster scrape sources."
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Example:
  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
  python3 scripts/check_prometheus_metrics.py --port 9090
        """,
    )
    p.add_argument(
        "--port",
        "-p",
        type=int,
        required=True,
        help="Local port where Prometheus is port-forwarded (e.g. 9090)",
    )
    p.add_argument(
        "--host",
        default="127.0.0.1",
        help="Host for port-forwarded Prometheus (default: 127.0.0.1)",
    )
    p.add_argument(
        "--context",
        help="kubectl context (default: current context)",
    )
    p.add_argument(
        "--include-optional",
        action="store_true",
        help="Also check optional metrics (e.g. Karpenter)",
    )
    p.add_argument(
        "--timeout",
        type=float,
        default=30.0,
        help="HTTP timeout seconds for Prometheus API (default: 30)",
    )
    p.add_argument(
        "--quiet",
        "-q",
        action="store_true",
        help="Suppress progress logs on stderr",
    )
    return p.parse_args()


def main() -> int:
    args = parse_args()
    quiet = args.quiet
    prom_url = f"http://{args.host}:{args.port}"

    log_progress("CruiseKube Prometheus metrics check — starting", quiet=quiet)

    log_progress("Resolving kubectl context...", quiet=quiet)
    try:
        context = args.context or current_context()
        log_progress(f"  context: {context}", quiet=quiet)
    except subprocess.CalledProcessError as exc:
        print(f"Failed to get kubectl context: {exc}", file=sys.stderr)
        return 2

    client = PrometheusClient(prom_url, timeout=args.timeout)

    log_progress(f"Connecting to Prometheus at {prom_url} ...", quiet=quiet)
    try:
        client._get("/api/v1/status/config")
        log_progress("  Prometheus API reachable", quiet=quiet)
        log_progress(f"  service-discovery UI: {client.ui_service_discovery}", quiet=quiet)
        log_progress(f"  targets UI:           {client.ui_targets}", quiet=quiet)
    except RuntimeError as exc:
        print(
            red(f"Prometheus not reachable at {prom_url}.\n")
            + "Start port-forward first, e.g.:\n"
            + "  kubectl port-forward -n <namespace> svc/<prometheus-service> "
            f"{args.port}:9090",
            file=sys.stderr,
        )
        print(exc, file=sys.stderr)
        return 2

    log_progress("Loading scrape targets from /api/v1/targets ...", quiet=quiet)
    try:
        active_targets, dropped_targets = client.targets_state()
        log_progress(
            f"  {len(active_targets)} active, {len(dropped_targets)} dropped target(s)",
            quiet=quiet,
        )
        sd_report = build_service_discovery_report(
            client, active_targets, dropped_targets, quiet=quiet
        )
    except RuntimeError as exc:
        print(red(str(exc)), file=sys.stderr)
        return 2

    log_progress("Inspecting cluster exporters (kubectl) ...", quiet=quiet)
    try:
        cluster_sources = inspect_cluster_sources(context)
        for key in ("kube-state-metrics", "node-exporter", "kubelet"):
            info = cluster_sources[key]
            log_progress(f"  {key}: {info.summary}", quiet=quiet)
    except RuntimeError as exc:
        log_progress(f"  warning: {exc}", quiet=quiet)
        print(yellow(f"Warning: cluster inspection failed: {exc}"), file=sys.stderr)
        cluster_sources = {
            k: ClusterSourceInfo(k, False, "unknown (kubectl error)")
            for k in ("kube-state-metrics", "node-exporter", "kubelet")
        }

    try:
        results = run_metric_checks(
            client, args.include_optional, quiet=quiet
        )
    except RuntimeError as exc:
        print(red(str(exc)), file=sys.stderr)
        return 2

    log_progress("Building report...", quiet=quiet)
    exit_code = print_report(
        prom_url,
        context,
        results,
        cluster_sources,
        active_targets,
        sd_report,
        args.include_optional,
    )
    log_progress(
        "Done — "
        + ("all required metrics present" if exit_code == 0 else "missing metrics — see report above"),
        quiet=quiet,
    )
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
