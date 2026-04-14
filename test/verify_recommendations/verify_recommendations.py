"""
Verify CruiseKube recommendations against actual pod/container resources in the cluster.

Fetches recommendations from PostgreSQL (pod_resource_recommendations), compares
CPU and Memory (request/limit) with the live Kubernetes pod specs, and returns
mismatches for use in scripts or notebooks.
"""

from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

# Bytes per MB as used by CruiseKube (Go: BytesToMBDivisor = 1_000_000)
BYTES_PER_MB = 1_000_000
FLOAT_EPSILON = 1e-6


@dataclass
class RecommendationRow:
    """One row from pod_resource_recommendations."""
    workload_id: str
    node_name: str
    namespace: str
    pod: str
    container: str
    recommendation: dict[str, Any]  # cpu_request, memory_request, cpu_limit, memory_limit (floats)


@dataclass
class Mismatch:
    """A single recommendation vs actual mismatch (or error)."""
    namespace: str
    pod: str
    container: str
    workload_id: str
    node_name: str
    error: str = ""
    cpu_request_diff: bool = False
    memory_request_diff: bool = False
    cpu_limit_diff: bool = False
    memory_limit_diff: bool = False
    recommended_cpu_request: float = 0.0
    actual_cpu_request: float = 0.0
    recommended_memory_request: float = 0.0
    actual_memory_request: float = 0.0
    recommended_cpu_limit: float = 0.0
    actual_cpu_limit: float = 0.0
    recommended_memory_limit: float = 0.0
    actual_memory_limit: float = 0.0

    def has_diff(self) -> bool:
        return (
            self.cpu_request_diff or self.memory_request_diff
            or self.cpu_limit_diff or self.memory_limit_diff
        )

    def __str__(self) -> str:
        if self.error:
            return f"{self.namespace}/{self.pod} container={self.container} workload={self.workload_id}: {self.error}"
        lines = [
            f"{self.namespace}/{self.pod} container={self.container} workload={self.workload_id} node={self.node_name}"
        ]
        if self.cpu_request_diff:
            lines.append(f"  CPU request:  recommended={self.recommended_cpu_request:.4f} actual={self.actual_cpu_request:.4f}")
        if self.memory_request_diff:
            lines.append(f"  Memory request: recommended={self.recommended_memory_request:.2f} MB actual={self.actual_memory_request:.2f} MB")
        if self.cpu_limit_diff:
            lines.append(f"  CPU limit:  recommended={self.recommended_cpu_limit:.4f} actual={self.actual_cpu_limit:.4f}")
        if self.memory_limit_diff:
            lines.append(f"  Memory limit: recommended={self.recommended_memory_limit:.2f} MB actual={self.actual_memory_limit:.2f} MB")
        return "\n".join(lines)


def load_db_config(config_path: str | None = None) -> dict[str, Any]:
    """Load DB config from YAML file or environment."""
    if config_path and Path(config_path).exists():
        try:
            import yaml
            with open(config_path) as f:
                data = yaml.safe_load(f) or {}
            db = data.get("db") or {}
            return {
                "host": os.environ.get("DB_HOST", db.get("host", "localhost")),
                "port": int(os.environ.get("DB_PORT", db.get("port", 5432))),
                "database": os.environ.get("DB_NAME", db.get("database", "cruisekube")),
                "user": os.environ.get("DB_USER", db.get("username", "")),
                "password": os.environ.get("DB_PASSWORD", db.get("password", "")),
                "sslmode": os.environ.get("DB_SSLMODE", db.get("sslmode", "disable")),
            }
        except Exception as e:
            raise RuntimeError(f"Failed to load config from {config_path}: {e}") from e
    return {
        "host": os.environ.get("DB_HOST", "localhost"),
        "port": int(os.environ.get("DB_PORT", "5432")),
        "database": os.environ.get("DB_NAME", "cruisekube"),
        "user": os.environ.get("DB_USER", ""),
        "password": os.environ.get("DB_PASSWORD", ""),
        "sslmode": os.environ.get("DB_SSLMODE", "disable"),
    }


def _kubernetes_clients(kubeconfig_path: str | None = None) -> tuple[Any, Any]:
    """Load kubeconfig (or in-cluster) and return (CoreV1Api, AppsV1Api)."""
    from kubernetes import client, config

    kubeconfig = kubeconfig_path or os.environ.get("KUBECONFIG") or str(Path.home() / ".kube" / "config")
    try:
        if Path(kubeconfig).expanduser().exists():
            config.load_kube_config(config_file=kubeconfig)
        else:
            config.load_incluster_config()
    except Exception as e:
        raise RuntimeError(f"Failed to load kube config: {e}") from e
    return client.CoreV1Api(), client.AppsV1Api()


def fetch_recommendations(cluster_id: str, config_path: str | None = None) -> list[RecommendationRow]:
    """Fetch all pod recommendations for a cluster from PostgreSQL."""
    import psycopg2

    cfg = load_db_config(config_path)
    conn = psycopg2.connect(
        host=cfg["host"],
        port=cfg["port"],
        dbname=cfg["database"],
        user=cfg["user"],
        password=cfg["password"],
        connect_timeout=10,
    )
    conn.autocommit = True
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT workload_id, node_name, namespace, pod, container, recommendation
                FROM pod_resource_recommendations
                WHERE cluster_id = %s
                """,
                (cluster_id,),
            )
            rows = []
            for r in cur.fetchall():
                rec = json.loads(r[5]) if isinstance(r[5], str) else r[5]
                rows.append(
                    RecommendationRow(
                        workload_id=r[0],
                        node_name=r[1],
                        namespace=r[2],
                        pod=r[3],
                        container=r[4],
                        recommendation=rec,
                    )
                )
            return rows
    finally:
        conn.close()


def _quantity_to_cpu(q: Any) -> float:
    """Convert Kubernetes quantity to CPU cores (float)."""
    from kubernetes.utils.quantity import parse_quantity
    if q is None:
        return 0.0
    s = str(q).strip()
    if not s:
        return 0.0
    # parse_quantity returns Decimal; for CPU '100m' -> 0.1
    val = parse_quantity(s)
    return float(val)


def _quantity_to_mb(q: Any) -> float:
    """Convert Kubernetes memory quantity to MB (CruiseKube uses 1e6 bytes = 1 MB)."""
    from kubernetes.utils.quantity import parse_quantity
    if q is None:
        return 0.0
    s = str(q).strip()
    if not s:
        return 0.0
    # parse_quantity returns value in bytes for memory
    bytes_val = float(parse_quantity(s))
    return bytes_val / BYTES_PER_MB


def _float_equal(a: float, b: float) -> bool:
    if a == 0 and b == 0:
        return True
    return abs(a - b) < FLOAT_EPSILON


def _actual_resources_from_pod(
    pod: Any,
    namespace: str,
    pod_name: str,
    container_name: str,
) -> tuple[float, float, float, float]:
    """
    Read CPU/memory request/limit for a container from an already-fetched Pod object.
    Returns (cpu_req, mem_req_mb, cpu_lim, mem_lim_mb). Missing values are 0.0.
    """
    container = None
    for c in pod.spec.containers:
        if c.name == container_name:
            container = c
            break
    if container is None:
        for c in pod.spec.init_containers or []:
            if c.name == container_name:
                container = c
                break
    if container is None:
        raise ValueError(f"Container {container_name} not found in pod {namespace}/{pod_name}")

    req = container.resources.requests or {}
    lim = container.resources.limits or {}
    cpu_req = _quantity_to_cpu(req.get("cpu"))
    mem_req = _quantity_to_mb(req.get("memory"))
    cpu_lim = _quantity_to_cpu(lim.get("cpu"))
    mem_lim = _quantity_to_mb(lim.get("memory"))
    return cpu_req, mem_req, cpu_lim, mem_lim


def get_actual_resources(
    v1: Any,
    namespace: str,
    pod_name: str,
    container_name: str,
) -> tuple[float, float, float, float]:
    """
    Get actual CPU request, memory request, CPU limit, memory limit for a container.
    Returns (cpu_req, mem_req_mb, cpu_lim, mem_lim_mb). Missing values are 0.0.
    """
    pod = v1.read_namespaced_pod(name=pod_name, namespace=namespace)
    return _actual_resources_from_pod(pod, namespace, pod_name, container_name)


def prefetch_pods_by_key(
    v1: Any,
    rows: list[RecommendationRow],
) -> tuple[dict[tuple[str, str], Any], dict[tuple[str, str], str]]:
    """
    One read_namespaced_pod per unique (namespace, pod).
    Returns (pods, fetch_errors) where fetch_errors maps key -> error message.
    """
    unique_keys = {(r.namespace, r.pod) for r in rows}
    pods: dict[tuple[str, str], Any] = {}
    errors: dict[tuple[str, str], str] = {}
    for namespace, pod_name in unique_keys:
        try:
            pods[(namespace, pod_name)] = v1.read_namespaced_pod(
                name=pod_name, namespace=namespace
            )
        except Exception as e:
            errors[(namespace, pod_name)] = str(e)
    return pods, errors


def compare_one(
    row: RecommendationRow,
    v1: Any,
    pod: Any | None = None,
    pod_fetch_error: str | None = None,
) -> Mismatch | None:
    """
    Compare one recommendation row with the live pod. Returns a Mismatch if different or error.
    If ``pod`` is provided, it must be the Pod for (row.namespace, row.pod); avoids an API call.
    If ``pod_fetch_error`` is set, that error is recorded for this row (failed prefetch).
    """
    rec = row.recommendation
    rec_cpu_req = float(rec.get("cpu_request", 0))
    rec_mem_req = float(rec.get("memory_request", 0))
    rec_cpu_lim = float(rec.get("cpu_limit", 0))
    rec_mem_lim = float(rec.get("memory_limit", 0))

    if pod_fetch_error is not None:
        return Mismatch(
            namespace=row.namespace,
            pod=row.pod,
            container=row.container,
            workload_id=row.workload_id,
            node_name=row.node_name,
            error=pod_fetch_error,
        )

    try:
        if pod is not None:
            actual_cpu_req, actual_mem_req, actual_cpu_lim, actual_mem_lim = (
                _actual_resources_from_pod(pod, row.namespace, row.pod, row.container)
            )
        else:
            actual_cpu_req, actual_mem_req, actual_cpu_lim, actual_mem_lim = get_actual_resources(
                v1, row.namespace, row.pod, row.container
            )
    except Exception as e:
        return Mismatch(
            namespace=row.namespace,
            pod=row.pod,
            container=row.container,
            workload_id=row.workload_id,
            node_name=row.node_name,
            error=str(e),
        )

    m = Mismatch(
        namespace=row.namespace,
        pod=row.pod,
        container=row.container,
        workload_id=row.workload_id,
        node_name=row.node_name,
    )
    if not _float_equal(rec_cpu_req, actual_cpu_req):
        m.cpu_request_diff = True
        m.recommended_cpu_request = rec_cpu_req
        m.actual_cpu_request = actual_cpu_req
    if not _float_equal(rec_mem_req, actual_mem_req):
        m.memory_request_diff = True
        m.recommended_memory_request = rec_mem_req
        m.actual_memory_request = actual_mem_req
    if not _float_equal(rec_cpu_lim, actual_cpu_lim):
        m.cpu_limit_diff = True
        m.recommended_cpu_limit = rec_cpu_lim
        m.actual_cpu_limit = actual_cpu_lim
    if not _float_equal(rec_mem_lim, actual_mem_lim):
        m.memory_limit_diff = True
        m.recommended_memory_limit = rec_mem_lim
        m.actual_memory_limit = actual_mem_lim

    if m.has_diff():
        return m
    return None


def parse_workload_key(workload_id: str) -> tuple[str, str, str] | None:
    """Parse workload_id as kind:namespace:name (same as Go GetWorkloadKey)."""
    parts = workload_id.split(":")
    if len(parts) != 3:
        return None
    return parts[0], parts[1], parts[2]


def get_workload_key(kind: str, namespace: str, name: str) -> str:
    return f"{kind}:{namespace}:{name}"


def _pod_matches_label_selector(pod_labels: dict[str, str], selector: Any) -> bool:
    """Return True if pod labels satisfy a v1.LabelSelector (subset of server semantics)."""
    if selector is None:
        return False
    ml = getattr(selector, "match_labels", None) or {}
    for k, v in ml.items():
        if pod_labels.get(k) != v:
            return False
    for req in getattr(selector, "match_expressions", None) or []:
        key = req.key
        op = req.operator
        vals = list(req.values or [])
        if op == "In":
            if pod_labels.get(key) not in vals:
                return False
        elif op == "NotIn":
            if pod_labels.get(key) in vals:
                return False
        elif op == "Exists":
            if key not in pod_labels:
                return False
        elif op == "DoesNotExist":
            if key in pod_labels:
                return False
        else:
            return False
    return True


def list_cluster_workloads_ordered(apps_v1: Any) -> list[tuple[str, str, str, str, Any]]:
    """
    List Deployments, StatefulSets, then DaemonSets with a pod label selector (same order as CruiseKube).
    Returns tuples (kind, namespace, name, workload_key, label_selector_object).
    """
    out: list[tuple[str, str, str, str, Any]] = []
    for list_fn, kind in (
        (apps_v1.list_deployment_for_all_namespaces, "Deployment"),
        (apps_v1.list_stateful_set_for_all_namespaces, "StatefulSet"),
        (apps_v1.list_daemon_set_for_all_namespaces, "DaemonSet"),
    ):
        for obj in list_fn().items:
            sel = obj.spec.selector
            if sel is None:
                continue
            ns, name = obj.metadata.namespace, obj.metadata.name
            wk = get_workload_key(kind, ns, name)
            out.append((kind, ns, name, wk, sel))
    return out


def scheduled_running_pods(v1: Any) -> list[Any]:
    """Pods with spec.nodeName set and phase Running (aligned with CreateStats)."""
    pods = []
    for pod in v1.list_pod_for_all_namespaces().items:
        if pod.spec.node_name and pod.status.phase == "Running":
            pods.append(pod)
    return pods


def build_pod_to_workload_map(
    pods: list[Any],
    workloads_ordered: list[tuple[str, str, str, str, Any]],
) -> dict[tuple[str, str], str]:
    """First selector match wins (same iteration order as CruiseKube)."""
    m: dict[tuple[str, str], str] = {}
    for pod in pods:
        key = (pod.metadata.namespace, pod.metadata.name)
        labels = pod.metadata.labels or {}
        if not isinstance(labels, dict):
            labels = dict(labels)
        for _kind, ns, _name, workload_key, selector in workloads_ordered:
            if ns != pod.metadata.namespace:
                continue
            if _pod_matches_label_selector(labels, selector):
                m[key] = workload_key
                break
    return m


def fetch_workload_rows(cluster_id: str, config_path: str | None = None) -> dict[str, dict[str, Any]]:
    """workload_id -> parsed stats JSON from workloads.stats."""
    import psycopg2

    cfg = load_db_config(config_path)
    conn = psycopg2.connect(
        host=cfg["host"],
        port=cfg["port"],
        dbname=cfg["database"],
        user=cfg["user"],
        password=cfg["password"],
        connect_timeout=10,
    )
    conn.autocommit = True
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT workload_id, stats FROM workloads WHERE cluster_id = %s",
                (cluster_id,),
            )
            out: dict[str, dict[str, Any]] = {}
            for wid, stats_raw in cur.fetchall():
                if isinstance(stats_raw, str):
                    out[wid] = json.loads(stats_raw)
                else:
                    out[wid] = stats_raw or {}
            return out
    finally:
        conn.close()


def fetch_distinct_rec_workload_ids(cluster_id: str, config_path: str | None = None) -> set[str]:
    import psycopg2

    cfg = load_db_config(config_path)
    conn = psycopg2.connect(
        host=cfg["host"],
        port=cfg["port"],
        dbname=cfg["database"],
        user=cfg["user"],
        password=cfg["password"],
        connect_timeout=10,
    )
    conn.autocommit = True
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT DISTINCT workload_id FROM pod_resource_recommendations WHERE cluster_id = %s",
                (cluster_id,),
            )
            return {r[0] for r in cur.fetchall()}
    finally:
        conn.close()


def _container_resources_from_spec(container: Any) -> tuple[float, float, float, float]:
    """CPU cores and memory MB for one container (aligned with CreateStats setResourceRequestAndLimit)."""
    req = container.resources.requests or {}
    lim = container.resources.limits or {}
    return (
        _quantity_to_cpu(req.get("cpu")),
        _quantity_to_mb(req.get("memory")),
        _quantity_to_cpu(lim.get("cpu")),
        _quantity_to_mb(lim.get("memory")),
    )


def _template_resources_by_container_name(apps_v1: Any, kind: str, namespace: str, name: str) -> dict[str, tuple[float, float, float, float]]:
    """name -> (cpu_req, mem_req_mb, cpu_lim, mem_lim) from workload pod template."""
    if kind == "Deployment":
        obj = apps_v1.read_namespaced_deployment(name, namespace)
        template = obj.spec.template
    elif kind == "StatefulSet":
        obj = apps_v1.read_namespaced_stateful_set(name, namespace)
        template = obj.spec.template
    elif kind == "DaemonSet":
        obj = apps_v1.read_namespaced_daemon_set(name, namespace)
        template = obj.spec.template
    else:
        return {}

    by_name: dict[str, tuple[float, float, float, float]] = {}
    for c in template.spec.containers or []:
        by_name[c.name] = _container_resources_from_spec(c)
    for c in template.spec.init_containers or []:
        rp = getattr(c, "restart_policy", None)
        if rp == "Always":
            by_name[c.name] = _container_resources_from_spec(c)
        else:
            by_name[c.name] = _container_resources_from_spec(c)
    return by_name


@dataclass
class StatResourceMismatch:
    workload_id: str
    container: str
    field: str
    stats_value: float
    template_value: float


@dataclass
class RecWorkloadMismatch:
    namespace: str
    pod: str
    workload_id_in_db: str
    workload_id_expected: str


@dataclass
class WorkloadAlignmentReport:
    cluster_workload_keys: set[str]
    db_workload_keys: set[str]
    in_cluster_not_in_db: list[str]
    in_db_not_in_cluster: list[str]
    rec_workload_ids: set[str]
    rec_workload_id_not_in_cluster: list[str]
    rec_workload_id_not_in_workloads_table: list[str]
    rec_row_workload_mismatches: list[RecWorkloadMismatch]
    rec_pods_no_workload_mapping: list[tuple[str, str]]
    stat_resource_mismatches: list[StatResourceMismatch]


def run_workload_db_cluster_alignment(
    cluster_id: str,
    rows: list[RecommendationRow],
    config_path: str | None = None,
    kubeconfig_path: str | None = None,
) -> WorkloadAlignmentReport:
    """
    Compare workloads / pod_resource_recommendations tables with the cluster.

    - Cluster workloads: Deployment, StatefulSet, DaemonSet with selectors (same as CreateStats).
    - DB workloads.workload_id set should match that cluster set (order-insensitive).
    - Each recommendation row's workload_id should match the workload derived from pod labels.
    - workloads.stats original_container_resources should match the live workload pod template.
    """
    v1, apps_v1 = _kubernetes_clients(kubeconfig_path)

    workloads_ordered = list_cluster_workloads_ordered(apps_v1)
    cluster_keys = {entry[3] for entry in workloads_ordered}

    db_stats_by_wid = fetch_workload_rows(cluster_id, config_path)
    db_keys = set(db_stats_by_wid.keys())

    pods = scheduled_running_pods(v1)
    pod_to_wk = build_pod_to_workload_map(pods, workloads_ordered)

    rec_wids = fetch_distinct_rec_workload_ids(cluster_id, config_path)
    if rows:
        rec_wids = rec_wids | {r.workload_id for r in rows}

    in_cluster_not_in_db = sorted(cluster_keys - db_keys)
    in_db_not_in_cluster = sorted(db_keys - cluster_keys)

    rec_not_cluster = sorted(rec_wids - cluster_keys)
    rec_not_workloads_tbl = sorted(rec_wids - db_keys)

    rec_mismatches: list[RecWorkloadMismatch] = []
    unmapped_pods_set: set[tuple[str, str]] = set()
    for row in rows:
        key = (row.namespace, row.pod)
        expected = pod_to_wk.get(key)
        if expected is None:
            unmapped_pods_set.add(key)
            continue
        if row.workload_id != expected:
            rec_mismatches.append(
                RecWorkloadMismatch(
                    namespace=row.namespace,
                    pod=row.pod,
                    workload_id_in_db=row.workload_id,
                    workload_id_expected=expected,
                )
            )
    unmapped_pods = sorted(unmapped_pods_set)

    stat_mismatches: list[StatResourceMismatch] = []
    for wid, stats in db_stats_by_wid.items():
        parsed = parse_workload_key(wid)
        if not parsed:
            continue
        kind, namespace, name = parsed
        if kind not in ("Deployment", "StatefulSet", "DaemonSet"):
            continue
        if wid not in cluster_keys:
            continue
        ocr = stats.get("original_container_resources") or []
        try:
            tmpl = _template_resources_by_container_name(apps_v1, kind, namespace, name)
        except Exception:
            continue
        for entry in ocr:
            cname = entry.get("name")
            if not cname or cname not in tmpl:
                continue
            cpu_r, mem_r, cpu_l, mem_l = tmpl[cname]
            if not _float_equal(float(entry.get("cpu_request", 0)), cpu_r):
                stat_mismatches.append(
                    StatResourceMismatch(wid, cname, "cpu_request", float(entry.get("cpu_request", 0)), cpu_r)
                )
            if not _float_equal(float(entry.get("memory_request", 0)), mem_r):
                stat_mismatches.append(
                    StatResourceMismatch(wid, cname, "memory_request", float(entry.get("memory_request", 0)), mem_r)
                )
            if not _float_equal(float(entry.get("cpu_limit", 0)), cpu_l):
                stat_mismatches.append(
                    StatResourceMismatch(wid, cname, "cpu_limit", float(entry.get("cpu_limit", 0)), cpu_l)
                )
            if not _float_equal(float(entry.get("memory_limit", 0)), mem_l):
                stat_mismatches.append(
                    StatResourceMismatch(wid, cname, "memory_limit", float(entry.get("memory_limit", 0)), mem_l)
                )

    return WorkloadAlignmentReport(
        cluster_workload_keys=cluster_keys,
        db_workload_keys=db_keys,
        in_cluster_not_in_db=in_cluster_not_in_db,
        in_db_not_in_cluster=in_db_not_in_cluster,
        rec_workload_ids=rec_wids,
        rec_workload_id_not_in_cluster=rec_not_cluster,
        rec_workload_id_not_in_workloads_table=rec_not_workloads_tbl,
        rec_row_workload_mismatches=rec_mismatches,
        rec_pods_no_workload_mapping=unmapped_pods,
        stat_resource_mismatches=stat_mismatches,
    )


def summarize_alignment(report: WorkloadAlignmentReport) -> str:
    lines = [
        f"Cluster workloads (Deploy/STS/DS): {len(report.cluster_workload_keys)}",
        f"DB workloads rows: {len(report.db_workload_keys)}",
        f"In cluster, not in DB: {len(report.in_cluster_not_in_db)}",
        f"In DB, not in cluster: {len(report.in_db_not_in_cluster)}",
        f"Distinct workload_ids in recommendations: {len(report.rec_workload_ids)}",
        f"Rec workload_id not in cluster set: {len(report.rec_workload_id_not_in_cluster)}",
        f"Rec workload_id missing from workloads table: {len(report.rec_workload_id_not_in_workloads_table)}",
        f"Recommendation rows with wrong workload_id vs pod labels: {len(report.rec_row_workload_mismatches)}",
        f"Recommendation pods with no selector match (excluded/static pods, etc.): {len(report.rec_pods_no_workload_mapping)}",
        f"workloads.stats vs live template resource mismatches: {len(report.stat_resource_mismatches)}",
    ]
    return "\n".join(lines)


def run_verification(
    cluster_id: str,
    config_path: str | None = None,
    kubeconfig_path: str | None = None,
) -> tuple[list[RecommendationRow], list[Mismatch]]:
    """
    Fetch recommendations from DB, compare with cluster, return (rows, mismatches).
    Mismatches include both comparison diffs and errors (pod/container not found, etc.).
    """
    rows = fetch_recommendations(cluster_id, config_path)
    if not rows:
        return rows, []

    v1, _apps = _kubernetes_clients(kubeconfig_path)
    pods_by_key, fetch_errors = prefetch_pods_by_key(v1, rows)
    mismatches: list[Mismatch] = []
    for row in rows:
        key = (row.namespace, row.pod)
        err = fetch_errors.get(key)
        pod = pods_by_key.get(key) if err is None else None
        result = compare_one(
            row,
            v1,
            pod=pod,
            pod_fetch_error=err,
        )
        if result is not None:
            mismatches.append(result)
    return rows, mismatches


def main() -> None:
    """CLI entrypoint."""
    import argparse
    parser = argparse.ArgumentParser(description="Verify CruiseKube recommendations vs cluster resources")
    parser.add_argument("--cluster-id", required=True, help="Cluster ID in the database")
    parser.add_argument("--config", default="", help="Path to config.yaml (optional)")
    parser.add_argument("--kubeconfig", default="", help="Path to kubeconfig (optional)")
    args = parser.parse_args()
    config_path = args.config or None
    kubeconfig = args.kubeconfig or None

    rows, mismatches = run_verification(args.cluster_id, config_path, kubeconfig)
    print(f"Fetched {len(rows)} recommendations for cluster {args.cluster_id}")
    if not mismatches:
        print("All recommendations match actual pod/container resources.")
        return
    print(f"\n--- {len(mismatches)} mismatch(es) ---\n")
    for m in mismatches:
        print(m)
    raise SystemExit(1)


if __name__ == "__main__":
    main()
