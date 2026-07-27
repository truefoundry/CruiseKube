#!/usr/bin/env python3
"""
CruiseKube diagnostics bundle.

Mirrors the controller's **preflight** check and packages the result for sharing:

  1. Prometheus connectivity & version (build info).
  2. Kubernetes server + per-node kubelet versions, and the Prometheus version,
     each against the minimums CruiseKube requires.
  3. Every metric CruiseKube relies on — present or not — plus each metric's
     distinct label names.
  4. Controller logs (two hours by default; 1h minimum).
  5. A masked report written to a log file: key identifiers (namespace, node,
     workload names, IPs) are pseudonymized and secrets/tokens/emails redacted.

Runs with a step-by-step terminal UI so you can see exactly what it is doing.
Only the Python standard library and a working ``kubectl`` are required.

Prometheus is read over a local port-forward (same model as
``check_prometheus_metrics.py``):

    kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
    python3 scripts/cruisekube_diagnostics.py --port 9090

Progress/UI is printed to **stderr**; the masked report is written to the
``--output`` file (default: ``cruisekube-diagnostics-<ts>.log``).
"""

from __future__ import annotations

import argparse
import json
import re
import socket
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Callable, Dict, List, Optional, Tuple

# ---------------------------------------------------------------------------
# Policy — mirrors pkg/handlers (preflight). Keep in sync with the Go code.
# ---------------------------------------------------------------------------
MIN_KUBE_VERSION = "1.33.0"          # per-node kubelet
MIN_KUBERNETES_VERSION = "1.33.0"    # control plane
MIN_PROMETHEUS_VERSION = "2.30.0"
METRIC_LOOKBACK = "15m"
MIN_SINCE_SECONDS = 3600             # "at least one hour" of logs


@dataclass(frozen=True)
class MetricProbe:
    metric: str
    job_matcher: str
    group: str
    required: bool


METRIC_PROBES: Tuple[MetricProbe, ...] = (
    # kube-state-metrics (always present when scraped)
    MetricProbe("kube_pod_info", 'job="kube-state-metrics"', "kube-state-metrics", True),
    MetricProbe("kube_pod_status_phase", 'job="kube-state-metrics"', "kube-state-metrics", True),
    MetricProbe("kube_pod_container_resource_requests", 'job="kube-state-metrics"', "kube-state-metrics", True),
    MetricProbe("kube_pod_container_status_restarts_total", 'job="kube-state-metrics"', "kube-state-metrics", True),
    MetricProbe("kube_node_status_allocatable", 'job="kube-state-metrics"', "kube-state-metrics", True),
    MetricProbe("kube_node_status_capacity", 'job="kube-state-metrics"', "kube-state-metrics", True),
    MetricProbe("kube_node_labels", 'job="kube-state-metrics"', "kube-state-metrics", True),
    # kube-state-metrics (legitimately zero-series on a healthy cluster)
    MetricProbe("kube_node_spec_taint", 'job="kube-state-metrics"', "kube-state-metrics", False),
    MetricProbe("kube_pod_container_status_last_terminated_reason", 'job="kube-state-metrics"', "kube-state-metrics", False),
    # cAdvisor / kubelet
    MetricProbe("container_cpu_usage_seconds_total", 'job=~"kubelet|kubernetes-nodes-cadvisor"', "cadvisor-kubelet", True),
    MetricProbe("container_memory_working_set_bytes", 'job=~"kubelet|kubernetes-nodes-cadvisor"', "cadvisor-kubelet", True),
    # node-exporter
    MetricProbe("node_load1", 'job="node-exporter"', "node-exporter", True),
    MetricProbe("node_cpu_seconds_total", 'job="node-exporter"', "node-exporter", True),
    # PSI pressure metrics — supported but OPTIONAL (non-blocking)
    MetricProbe("container_pressure_cpu_waiting_seconds_total", 'job=~"kubelet|kubernetes-nodes-cadvisor"', "psi", False),
    MetricProbe("container_pressure_memory_waiting_seconds_total", 'job=~"kubelet|kubernetes-nodes-cadvisor"', "psi", False),
    MetricProbe("node_pressure_cpu_waiting_seconds_total", 'job="node-exporter"', "psi", False),
    MetricProbe("node_pressure_memory_waiting_seconds_total", 'job="node-exporter"', "psi", False),
    # Karpenter (optional; empty until a disruption)
    MetricProbe("karpenter_nodeclaims_disrupted_total", "", "karpenter", False),
)


# ===========================================================================
# Terminal UI
# ===========================================================================
class UI:
    """Minimal, dependency-free step UI. Everything goes to stderr."""

    def __init__(self, stream=sys.stderr, color: Optional[bool] = None):
        self.stream = stream
        self.tty = stream.isatty()
        self.color = self.tty if color is None else color

    def _c(self, code: str, text: str) -> str:
        return f"\033[{code}m{text}\033[0m" if self.color else text

    def banner(self, title: str) -> None:
        line = "─" * (len(title) + 2)
        self.stream.write(f"\n{self._c('1;36', '┌' + line + '┐')}\n")
        self.stream.write(f"{self._c('1;36', '│ ' + title + ' │')}\n")
        self.stream.write(f"{self._c('1;36', '└' + line + '┘')}\n")
        self.stream.flush()

    def plan(self, steps: List[Tuple[str, str]]) -> None:
        self.stream.write(self._c("1", "\nThis script will:\n"))
        for i, (title, desc) in enumerate(steps, 1):
            self.stream.write(f"  {self._c('36', f'{i}.')} {self._c('1', title)} — {desc}\n")
        self.stream.write(
            self._c("2", "\nThe report is written masked: identifiers (namespace, node,\n"
                         "workload, IP) are pseudonymized and secrets/tokens/emails redacted.\n\n"))
        self.stream.flush()

    def note(self, text: str) -> None:
        self.stream.write(self._c("2", f"    {text}\n"))
        self.stream.flush()

    def step(self, idx: int, total: int, title: str) -> "Step":
        return Step(self, idx, total, title)


class Step:
    _FRAMES = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

    def __init__(self, ui: UI, idx: int, total: int, title: str):
        self.ui = ui
        self.label = f"[{idx}/{total}] {title}"
        self.status = "ok"
        self.summary = ""
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self._start = 0.0

    def ok(self, summary: str) -> None:
        self.status, self.summary = "ok", summary

    def warn(self, summary: str) -> None:
        self.status, self.summary = "warn", summary

    def fail(self, summary: str) -> None:
        self.status, self.summary = "fail", summary

    def __enter__(self) -> "Step":
        self._start = time.time()
        if self.ui.tty:
            def spin():
                i = 0
                while not self._stop.is_set():
                    frame = self._FRAMES[i % len(self._FRAMES)]
                    self.ui.stream.write(f"\r{self.ui._c('36', frame)} {self.label}   ")
                    self.ui.stream.flush()
                    i += 1
                    time.sleep(0.1)
            self._thread = threading.Thread(target=spin, daemon=True)
            self._thread.start()
        else:
            self.ui.stream.write(f"… {self.label}\n")
            self.ui.stream.flush()
        return self

    def __exit__(self, exc_type, exc, tb) -> bool:
        if exc is not None:
            self.status = "fail"
            self.summary = f"{exc_type.__name__}: {exc}"
        self._stop.set()
        if self._thread:
            self._thread.join()
        if self.ui.tty:
            self.ui.stream.write("\r\033[2K")  # clear spinner line
        icon = {"ok": self.ui._c("1;32", "✔"),
                "warn": self.ui._c("1;33", "⚠"),
                "fail": self.ui._c("1;31", "✖")}[self.status]
        dur = time.time() - self._start
        self.ui.stream.write(f"{icon} {self.label} — {self.summary} "
                             f"{self.ui._c('2', f'({dur:.1f}s)')}\n")
        self.ui.stream.flush()
        return True  # swallow the exception; a failed step must not abort the run


# ===========================================================================
# Masking / pseudonymization
# ===========================================================================
_SECRET_KEYS = (
    r"pass(?:word|wd)?|secret|token|api[_-]?key|access[_-]?key|"
    r"bearer[_-]?token|tfy[_-]?cluster[_-]?token|client[_-]?secret|private[_-]?key"
)

_REGEX_RULES: List[Tuple[re.Pattern, str]] = [
    (re.compile(r"://[^/\s:@]+:[^/\s:@]+@"), "://***REDACTED***@"),
    (re.compile(r'(?i)(authorization"?\s*[:=]\s*"?)([^"\r\n]+)'), r"\1***REDACTED***"),
    (re.compile(r"(?i)\b(bearer)\s+[A-Za-z0-9._~+/\-]+=*"), r"\1 ***REDACTED***"),
    (re.compile(r"\beyJ[A-Za-z0-9._\-]{10,}"), "***REDACTED_JWT***"),
    (re.compile(r'(?i)("[\w.\-]*(?:' + _SECRET_KEYS + r')"\s*:\s*")([^"]*)(")'), r"\1***REDACTED***\3"),
    (re.compile(r"(?i)([\w.\-]*(?:" + _SECRET_KEYS + r"))(\s*[:=]\s*)(\S+)"), r"\1\2***REDACTED***"),
    (re.compile(r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b"), "***REDACTED_EMAIL***"),
]
_IP_RULE = (re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"), "***IP***")

# Identifiers that are not sensitive and are more useful left readable.
_SYSTEM_NAMES = {
    "default", "kube-system", "kube-public", "kube-node-lease",
    "cruisekube-system", "cruisekube-metrics", "monitoring",
}


class Masker:
    """Pseudonymizes cluster identifiers consistently, then redacts secrets."""

    # An identifier-like token: a namespace/node/workload/pod name. Tokenizing
    # once and doing an O(1) dict lookup per token keeps masking linear in the
    # text size, independent of how many identifiers we pseudonymize (a cluster
    # can have thousands of workloads).
    _TOKEN_RE = re.compile(r"[A-Za-z0-9][\w.\-]{2,}")

    def __init__(self, mask_ips: bool = True, pseudonymize: bool = True):
        self.mask_ips = mask_ips
        self.pseudonymize = pseudonymize
        self._names: Dict[str, str] = {}   # real name -> placeholder
        self._pending: Dict[str, List[str]] = {}

    def register(self, kind: str, names: List[str]) -> None:
        bucket = self._pending.setdefault(kind, [])
        for n in names:
            n = (n or "").strip()
            if len(n) >= 4 and n not in _SYSTEM_NAMES and n not in bucket:
                bucket.append(n)

    def build(self) -> None:
        if not self.pseudonymize:
            return
        for kind in sorted(self._pending):
            for i, name in enumerate(sorted(set(self._pending[kind])), 1):
                self._names.setdefault(name, f"{kind}-{i}")

    def _pseudonymize(self, text: str) -> str:
        if not self._names:
            return text
        return self._TOKEN_RE.sub(
            lambda mo: self._names.get(mo.group(0), mo.group(0)), text)

    def mask(self, text: str) -> str:
        if not text:
            return text
        text = self._pseudonymize(text)             # identifiers first
        for pattern, repl in _REGEX_RULES:          # then redact secrets/emails
            text = pattern.sub(repl, text)
        if self.mask_ips:
            text = _IP_RULE[0].sub(_IP_RULE[1], text)
        return text

    def legend(self) -> str:
        if not self._names:
            return "(no identifiers pseudonymized)"
        by_kind: Dict[str, int] = {}
        for placeholder in self._names.values():
            kind = placeholder.rsplit("-", 1)[0]
            by_kind[kind] = by_kind.get(kind, 0) + 1
        return ", ".join(f"{v} {k}" for k, v in sorted(by_kind.items()))


# ===========================================================================
# Version helpers
# ===========================================================================
def parse_version(value: str) -> Tuple[int, int, int]:
    v = value.strip().lstrip("vV").split("-")[0].split("+")[0]
    nums: List[int] = []
    for part in v.split(".")[:3]:
        m = re.match(r"\d+", part)
        nums.append(int(m.group()) if m else 0)
    while len(nums) < 3:
        nums.append(0)
    return nums[0], nums[1], nums[2]


def at_least(value: str, minimum: str) -> bool:
    return parse_version(value) >= parse_version(minimum)


def parse_duration_seconds(value: str) -> int:
    matches = re.findall(r"(\d+)\s*([smhd])", value.strip().lower())
    if not matches:
        raise ValueError(f"invalid duration: {value!r}")
    unit = {"s": 1, "m": 60, "h": 3600, "d": 86400}
    return sum(int(n) * unit[u] for n, u in matches)


# ===========================================================================
# kubectl / Prometheus IO
# ===========================================================================
def run(cmd: List[str], timeout: int = 120) -> Tuple[int, str, str]:
    proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          text=True, timeout=timeout, check=False)
    return proc.returncode, proc.stdout, proc.stderr


def kubectl(ns: Optional[str], context: Optional[str], *args: str,
            request_timeout: str = "15s") -> List[str]:
    cmd = ["kubectl", "--request-timeout", request_timeout]
    if ns:
        cmd += ["-n", ns]
    if context:
        cmd += ["--context", context]
    return cmd + list(args)


class Prometheus:
    def __init__(self, host: str, port: int):
        self.base = f"http://{host}:{port}"

    def _get(self, path: str, params: Dict[str, object]) -> dict:
        url = f"{self.base}{path}?{urllib.parse.urlencode(params, doseq=True)}"
        with urllib.request.urlopen(url, timeout=30) as resp:
            payload = json.loads(resp.read().decode("utf-8", errors="replace"))
        if payload.get("status") != "success":
            raise RuntimeError(payload.get("error", "prometheus query failed"))
        return payload["data"]

    def buildinfo(self) -> dict:
        with urllib.request.urlopen(f"{self.base}/api/v1/status/buildinfo", timeout=15) as resp:
            return json.loads(resp.read().decode("utf-8", errors="replace")).get("data", {})

    def instant(self, query: str) -> list:
        return self._get("/api/v1/query", {"query": query}).get("result", [])

    def label_names(self, selector: str, lookback_seconds: int) -> List[str]:
        now = time.time()
        data = self._get("/api/v1/labels", {
            "match[]": selector,
            "start": f"{now - lookback_seconds:.0f}",
            "end": f"{now:.0f}",
        })
        return [n for n in data if n != "__name__"]


# ===========================================================================
# Report data model
# ===========================================================================
@dataclass
class MetricResult:
    metric: str
    group: str
    required: bool
    present: bool = False
    series: int = 0
    labels: List[str] = field(default_factory=list)
    error: str = ""


@dataclass
class Report:
    generated_at: str
    prom_connected: bool = False
    prom_version: str = ""
    prom_error: str = ""
    k8s_version: str = ""
    k8s_ok: bool = False
    nodes: List[Tuple[str, str, bool]] = field(default_factory=list)  # (name, kubelet, ok)
    metrics: List[MetricResult] = field(default_factory=list)
    logs: str = ""
    logs_error: str = ""


# ===========================================================================
# Steps
# ===========================================================================
def step_prometheus(prom: Prometheus, report: Report, st: Step) -> None:
    try:
        info = prom.buildinfo()
        report.prom_connected = True
        report.prom_version = info.get("version", "")
    except Exception as exc:  # noqa: BLE001
        # Fall back to a trivial query to distinguish "no buildinfo" from "down".
        try:
            prom.instant("vector(1)")
            report.prom_connected = True
            report.prom_error = f"buildinfo unavailable ({exc}); version unknown"
            st.warn(f"connected at {prom.base} (version unknown)")
            return
        except Exception as exc2:  # noqa: BLE001
            report.prom_error = f"failed to reach Prometheus at {prom.base}: {exc2}"
            st.fail(report.prom_error)
            return
    ok = report.prom_version and at_least(report.prom_version, MIN_PROMETHEUS_VERSION)
    if ok:
        st.ok(f"{prom.base} — Prometheus {report.prom_version} (>= {MIN_PROMETHEUS_VERSION})")
    else:
        st.warn(f"{prom.base} — Prometheus {report.prom_version or '?'} "
                f"(below {MIN_PROMETHEUS_VERSION})")


def step_versions(context: Optional[str], report: Report, st: Step) -> None:
    rc, out, err = run(kubectl(None, context, "version", "-o", "json"))
    if rc == 0:
        try:
            report.k8s_version = json.loads(out).get("serverVersion", {}).get("gitVersion", "")
        except json.JSONDecodeError:
            report.k8s_version = ""
    report.k8s_ok = bool(report.k8s_version) and at_least(report.k8s_version, MIN_KUBERNETES_VERSION)

    rc, out, err = run(kubectl(None, context, "get", "nodes", "-o", "json"))
    below = 0
    if rc == 0:
        try:
            for item in json.loads(out).get("items", []):
                name = item.get("metadata", {}).get("name", "")
                kubelet = item.get("status", {}).get("nodeInfo", {}).get("kubeletVersion", "")
                ok = bool(kubelet) and at_least(kubelet, MIN_KUBE_VERSION)
                report.nodes.append((name, kubelet, ok))
                if not ok:
                    below += 1
        except json.JSONDecodeError:
            pass
    else:
        st.fail(f"kubectl get nodes failed: {err.strip()}")
        return

    k8s_state = "ok" if report.k8s_ok else f"below {MIN_KUBERNETES_VERSION}"
    summary = (f"k8s {report.k8s_version or '?'} ({k8s_state}); "
               f"{len(report.nodes)} nodes, {below} below {MIN_KUBE_VERSION}")
    (st.ok if report.k8s_ok and below == 0 else st.warn)(summary)


def step_metrics(prom: Prometheus, report: Report, st: Step) -> None:
    lookback_seconds = parse_duration_seconds(METRIC_LOOKBACK)
    found = 0
    for p in METRIC_PROBES:
        res = MetricResult(metric=p.metric, group=p.group, required=p.required)
        selector = p.metric if not p.job_matcher else f"{p.metric}{{{p.job_matcher}}}"
        try:
            result = prom.instant(f"count(last_over_time({selector}[{METRIC_LOOKBACK}]))")
            if result:
                res.series = int(float(result[0]["value"][1]))
                res.present = res.series > 0
            if res.present:
                found += 1
                try:
                    res.labels = prom.label_names(selector, lookback_seconds)
                except Exception as exc:  # noqa: BLE001
                    res.error = f"labels unavailable: {exc}"
            else:
                res.error = f"no series in last {METRIC_LOOKBACK}"
        except Exception as exc:  # noqa: BLE001
            res.error = f"query failed: {exc}"
        report.metrics.append(res)

    missing_required = [m.metric for m in report.metrics if m.required and not m.present]
    total = len(report.metrics)
    if missing_required:
        st.warn(f"{found}/{total} present; missing required: {', '.join(missing_required)}")
    else:
        st.ok(f"{found}/{total} present; all required metrics found")


def step_logs(ns: str, selector: str, since: str, context: Optional[str],
              report: Report, st: Step) -> None:
    rc, out, err = run(
        kubectl(ns, context, "logs", "-l", selector, f"--since={since}",
                "--all-containers=true", "--prefix=true", "--timestamps=true", "--tail=-1",
                request_timeout="120s"),
        timeout=300,
    )
    if rc != 0:
        report.logs_error = f"kubectl logs failed: {err.strip()}"
        st.fail(report.logs_error)
        return
    report.logs = out
    st.ok(f"{out.count(chr(10))} log lines (--since={since})")


def step_redact_and_write(report: Report, masker: Masker, ns: str, selector: str,
                          context: Optional[str], output: str, since: str, st: Step) -> None:
    # Gather identifiers to pseudonymize.
    def names_from(args: List[str]) -> List[str]:
        rc, out, _ = run(kubectl(None, context, *args))
        return out.split() if rc == 0 else []

    masker.register("node", [n for (n, _, _) in report.nodes])
    masker.register("ns", names_from(
        ["get", "ns", "-o", "jsonpath={range .items[*]}{.metadata.name} {end}"]))
    masker.register("workload", names_from(
        ["get", "deploy,statefulset,daemonset", "-A",
         "-o", "jsonpath={range .items[*]}{.metadata.name} {end}"]))
    masker.build()

    text = render_report(report, masker, ns, selector, since)
    with open(output, "w", encoding="utf-8") as fh:
        fh.write(text)
    st.ok(f"masked ({masker.legend()}) → {output}")


# ===========================================================================
# Report rendering
# ===========================================================================
def _bar(title: str) -> str:
    return f"\n{'=' * 78}\n== {title}\n{'=' * 78}\n"


def render_report(report: Report, masker: Masker, ns: str, selector: str, since: str) -> str:
    healthy = (report.prom_connected
               and report.prom_version and at_least(report.prom_version, MIN_PROMETHEUS_VERSION)
               and report.k8s_ok
               and all(ok for (_, _, ok) in report.nodes) and report.nodes
               and all(m.present for m in report.metrics if m.required))

    out: List[str] = []
    out.append("CruiseKube diagnostics report")
    out.append(f"generated_at : {report.generated_at}")
    out.append(f"namespace    : {ns}")
    out.append(f"selector     : {selector}")
    out.append(f"healthy      : {str(healthy).lower()}")
    out.append(f"masking      : identifiers pseudonymized ({masker.legend()}); "
               "secrets/tokens/emails/IPs redacted")

    out.append(_bar("PROMETHEUS"))
    out.append(f"connected : {report.prom_connected}")
    out.append(f"version   : {report.prom_version or '(unknown)'} "
               f"(min {MIN_PROMETHEUS_VERSION})")
    if report.prom_error:
        out.append(f"note      : {report.prom_error}")

    out.append(_bar("VERSIONS"))
    out.append(f"kubernetes server : {report.k8s_version or '(unknown)'} "
               f"(min {MIN_KUBERNETES_VERSION}) -> {'OK' if report.k8s_ok else 'BELOW MIN'}")
    out.append(f"nodes (min kubelet {MIN_KUBE_VERSION}):")
    if report.nodes:
        for name, kubelet, ok in report.nodes:
            out.append(f"  - {name:<40} {kubelet:<12} {'OK' if ok else 'BELOW MIN'}")
    else:
        out.append("  (no nodes found)")

    out.append(_bar("METRICS (present or not, with distinct labels)"))
    groups: Dict[str, List[MetricResult]] = {}
    for m in report.metrics:
        groups.setdefault(m.group, []).append(m)
    for group, items in groups.items():
        req_present = all(m.present for m in items if m.required)
        out.append(f"[{group}] {'OK' if req_present else 'INCOMPLETE'}")
        for m in items:
            mark = "present" if m.present else "MISSING"
            req = "required" if m.required else "optional"
            labels = ", ".join(m.labels) if m.labels else "-"
            line = f"  - {m.metric:<48} {mark:<8} {req:<8} series={m.series} labels=[{labels}]"
            if m.error:
                line += f"  ({m.error})"
            out.append(line)

    out.append(_bar(f"CONTROLLER LOGS (--since={since})"))
    out.append(report.logs_error if report.logs_error else report.logs)

    text = "\n".join(out) + "\n"
    return masker.mask(text)


# ===========================================================================
# Main
# ===========================================================================
def main() -> int:
    parser = argparse.ArgumentParser(
        description="CruiseKube diagnostics: preflight-style checks + logs, masked, with a step UI.")
    parser.add_argument("--port", "-p", type=int, default=9090,
                        help="local port where Prometheus is forwarded (default: 9090)")
    parser.add_argument("--host", default="127.0.0.1", help="Prometheus host (default: 127.0.0.1)")
    parser.add_argument("--namespace", default="cruisekube-system",
                        help="controller namespace (default: cruisekube-system)")
    parser.add_argument("--selector", default="app.kubernetes.io/name=controller",
                        help="controller pod selector (default: app.kubernetes.io/name=controller)")
    parser.add_argument("--since", default="2h",
                        help="log window; minimum 1h enforced (default: 2h)")
    parser.add_argument("--context", default=None, help="kubectl context (default: current)")
    parser.add_argument("--output", default=None,
                        help="report file (default: cruisekube-diagnostics-<ts>.log)")
    parser.add_argument("--no-mask-ips", action="store_true", help="do not mask IP addresses")
    parser.add_argument("--no-pseudonymize", action="store_true",
                        help="do not pseudonymize namespace/node/workload names")
    parser.add_argument("--no-color", action="store_true", help="disable colored output")
    args = parser.parse_args()

    try:
        if parse_duration_seconds(args.since) < MIN_SINCE_SECONDS:
            print(f"note: --since {args.since} below 1h minimum; using 1h", file=sys.stderr)
            args.since = "1h"
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    now = datetime.now(timezone.utc)
    output = args.output or f"cruisekube-diagnostics-{now:%Y%m%dT%H%M%SZ}.log"

    ui = UI(color=False if args.no_color else None)
    prom = Prometheus(args.host, args.port)
    report = Report(generated_at=now.isoformat())
    masker = Masker(mask_ips=not args.no_mask_ips, pseudonymize=not args.no_pseudonymize)

    ui.banner("CruiseKube Diagnostics")
    ui.plan([
        ("Prometheus connectivity & version", f"build info at {prom.base}"),
        ("Kubernetes & node versions", f"server + kubelets vs {MIN_KUBERNETES_VERSION}"),
        ("Metrics & labels", f"{len(METRIC_PROBES)} metrics, present-or-not, with label names"),
        ("Controller logs", f"{args.since} from {args.namespace} (1h minimum)"),
        ("Redact & write report", f"masked output -> {output}"),
    ])
    ui.note(f"Reading Prometheus at {prom.base} "
            f"(port-forward it first, e.g. kubectl port-forward svc/<prometheus> {args.port}:9090)")

    total = 5
    with ui.step(1, total, "Prometheus connectivity & version") as st:
        step_prometheus(prom, report, st)
    with ui.step(2, total, "Kubernetes & node versions") as st:
        step_versions(args.context, report, st)
    with ui.step(3, total, "Metrics & labels") as st:
        step_metrics(prom, report, st)
    with ui.step(4, total, "Controller logs") as st:
        step_logs(args.namespace, args.selector, args.since, args.context, report, st)
    with ui.step(5, total, "Redact & write report") as st:
        step_redact_and_write(report, masker, args.namespace, args.selector,
                              args.context, output, args.since, st)

    ui.stream.write(f"\n{ui._c('1;32', 'Done.')} Report written to {ui._c('1', output)}\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
