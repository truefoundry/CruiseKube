#!/usr/bin/env python3

import argparse
import json
import subprocess
import sys
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple


CPU_SCALE = Decimal(1000)
MEMORY_FACTORS = {
    "": Decimal(1),
    "n": Decimal("1e-9"),
    "u": Decimal("1e-6"),
    "m": Decimal("1e-3"),
    "k": Decimal(1000),
    "K": Decimal(1000),
    "M": Decimal(1000) ** 2,
    "G": Decimal(1000) ** 3,
    "T": Decimal(1000) ** 4,
    "P": Decimal(1000) ** 5,
    "E": Decimal(1000) ** 6,
    "Ki": Decimal(1024),
    "Mi": Decimal(1024) ** 2,
    "Gi": Decimal(1024) ** 3,
    "Ti": Decimal(1024) ** 4,
    "Pi": Decimal(1024) ** 5,
    "Ei": Decimal(1024) ** 6,
}
POD_TEMPLATE_KINDS = {
    "Deployment",
    "StatefulSet",
    "DaemonSet",
    "Job",
    "CronJob",
}
WORKLOAD_OWNER_KINDS = {
    "Deployment",
    "StatefulSet",
    "DaemonSet",
    "Job",
    "CronJob",
}
TOP_LEVEL_WORKLOAD_KINDS = (
    "deployments",
    "statefulsets",
    "daemonsets",
    "jobs",
    "cronjobs",
    "pods",
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Calculate total requested CPU/memory across live pods in a cluster and "
            "original requested CPU/memory from workload specs."
        )
    )
    parser.add_argument(
        "--original-source",
        choices=["cluster", "manifests"],
        default="cluster",
        help=(
            "Source for original requested totals. "
            "'cluster' reads workload specs from the cluster. "
            "'manifests' reads local manifest files."
        ),
    )
    parser.add_argument(
        "-f",
        "--manifest",
        dest="manifests",
        action="append",
        help="Manifest file or directory. Repeatable.",
    )
    parser.add_argument(
        "-n",
        "--namespace",
        dest="namespaces",
        action="append",
        help="Namespace to include. Repeatable. Default: all namespaces.",
    )
    parser.add_argument(
        "--selector",
        help="Optional label selector for live pod totals, for example app=my-app.",
    )
    parser.add_argument(
        "--context",
        help="Optional kubeconfig context to pass to kubectl.",
    )
    parser.add_argument(
        "--include-completed",
        action="store_true",
        help="Include pods in Succeeded or Failed phase in current totals.",
    )
    return parser.parse_args()


def run_kubectl(args: List[str]) -> Dict:
    proc = subprocess.run(
        ["kubectl", *args],
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr or proc.stdout)
        raise SystemExit(proc.returncode)
    return json.loads(proc.stdout)


def split_quantity(value: str) -> Tuple[Decimal, str]:
    for suffix in sorted(MEMORY_FACTORS, key=len, reverse=True):
        if suffix and value.endswith(suffix):
            number = value[: -len(suffix)]
            break
    else:
        suffix = ""
        number = value
    try:
        return Decimal(number), suffix
    except InvalidOperation as exc:
        raise ValueError(f"invalid quantity: {value}") from exc


def parse_cpu(value: Optional[str]) -> Decimal:
    if not value:
        return Decimal(0)
    if value.endswith("m"):
        number = value[:-1] or "0"
        return Decimal(number)
    return Decimal(value) * CPU_SCALE


def parse_memory(value: Optional[str]) -> Decimal:
    if not value:
        return Decimal(0)
    number, suffix = split_quantity(value)
    if suffix not in MEMORY_FACTORS:
        raise ValueError(f"unsupported memory suffix in quantity: {value}")
    return number * MEMORY_FACTORS[suffix]


def sum_container_requests(containers: List[Dict]) -> Tuple[Decimal, Decimal]:
    cpu_total = Decimal(0)
    memory_total = Decimal(0)
    for container in containers or []:
        requests = container.get("resources", {}).get("requests", {})
        cpu_total += parse_cpu(requests.get("cpu"))
        memory_total += parse_memory(requests.get("memory"))
    return cpu_total, memory_total


def pod_effective_requests(spec: Dict) -> Tuple[Decimal, Decimal]:
    regular_cpu, regular_memory = sum_container_requests(spec.get("containers", []))
    init_cpu_max = Decimal(0)
    init_memory_max = Decimal(0)
    for container in spec.get("initContainers", []) or []:
        requests = container.get("resources", {}).get("requests", {})
        init_cpu_max = max(init_cpu_max, parse_cpu(requests.get("cpu")))
        init_memory_max = max(init_memory_max, parse_memory(requests.get("memory")))
    return max(regular_cpu, init_cpu_max), max(regular_memory, init_memory_max)


def normalize_items(payload: Dict) -> List[Dict]:
    if payload.get("kind") == "List":
        return payload.get("items", [])
    return [payload]


def pod_template_spec(resource: Dict) -> Optional[Dict]:
    kind = resource.get("kind")
    if kind == "Pod":
        return resource.get("spec")
    if kind == "CronJob":
        return (
            resource.get("spec", {})
            .get("jobTemplate", {})
            .get("spec", {})
            .get("template", {})
            .get("spec")
        )
    if kind in POD_TEMPLATE_KINDS:
        return resource.get("spec", {}).get("template", {}).get("spec")
    return None


def manifest_replicas(resource: Dict) -> Optional[int]:
    kind = resource.get("kind")
    spec = resource.get("spec", {})
    if kind in {"Deployment", "StatefulSet", "ReplicaSet", "ReplicationController"}:
        return int(spec.get("replicas", 1))
    if kind in {"Pod", "Job", "CronJob"}:
        return 1
    if kind == "DaemonSet":
        return None
    return None


def cluster_workload_replicas(resource: Dict) -> Optional[int]:
    kind = resource.get("kind")
    spec = resource.get("spec", {})
    status = resource.get("status", {})
    if kind in {"Deployment", "StatefulSet"}:
        return int(spec.get("replicas", 1))
    if kind == "DaemonSet":
        return int(status.get("desiredNumberScheduled", 0))
    if kind == "Job":
        return int(spec.get("parallelism", 1))
    if kind == "CronJob":
        return 1
    if kind == "Pod":
        if resource.get("metadata", {}).get("ownerReferences"):
            return 0
        return 1
    return None


def namespace_allowed(namespace: Optional[str], allowed: Optional[Set[str]]) -> bool:
    if not allowed:
        return True
    return (namespace or "default") in allowed


def resource_key(resource: Dict) -> Tuple[str, str, str]:
    metadata = resource.get("metadata", {})
    return (
        resource.get("kind", ""),
        metadata.get("namespace", "default"),
        metadata.get("name", ""),
    )


def fetch_workload_objects(
    context: Optional[str],
    selector: Optional[str],
) -> List[Dict]:
    cmd = [
        "get",
        "pods,replicasets,deployments,statefulsets,daemonsets,jobs,cronjobs",
        "-A",
        "-o",
        "json",
    ]
    if context:
        cmd = ["--context", context, *cmd]
    if selector:
        cmd.extend(["-l", selector])
    payload = run_kubectl(cmd)
    return payload.get("items", [])


def resolve_workload_kind(
    resource: Dict,
    resource_index: Dict[Tuple[str, str, str], Dict],
    seen: Optional[Set[Tuple[str, str, str]]] = None,
) -> Optional[str]:
    key = resource_key(resource)
    if seen is None:
        seen = set()
    if key in seen:
        return None
    seen.add(key)

    kind = resource.get("kind")
    owners = resource.get("metadata", {}).get("ownerReferences", [])
    if kind == "Pod" and not owners:
        return "Pod"
    if kind in WORKLOAD_OWNER_KINDS:
        return kind

    namespace = resource.get("metadata", {}).get("namespace", "default")
    for owner in owners:
        owner_key = (owner.get("kind", ""), namespace, owner.get("name", ""))
        if owner_key in seen:
            continue
        if owner.get("kind") in WORKLOAD_OWNER_KINDS:
            return owner.get("kind")
        owner_resource = resource_index.get(owner_key)
        if owner_resource is None:
            continue
        resolved = resolve_workload_kind(owner_resource, resource_index, seen)
        if resolved is not None:
            return resolved
    return None


def allocatable_totals(context: Optional[str]) -> Tuple[Decimal, Decimal, int]:
    cmd = ["get", "nodes", "-o", "json"]
    if context:
        cmd = ["--context", context, *cmd]

    payload = run_kubectl(cmd)
    cpu_total = Decimal(0)
    memory_total = Decimal(0)
    node_count = 0

    for node in payload.get("items", []):
        allocatable = node.get("status", {}).get("allocatable", {})
        cpu_total += parse_cpu(allocatable.get("cpu"))
        memory_total += parse_memory(allocatable.get("memory"))
        node_count += 1

    return cpu_total, memory_total, node_count


def current_totals(
    context: Optional[str],
    namespaces: Optional[Set[str]],
    selector: Optional[str],
    include_completed: bool,
) -> Tuple[Decimal, Decimal, int]:
    cmd = ["get", "pods", "-A", "-o", "json"]
    if context:
        cmd = ["--context", context, *cmd]
    if selector:
        cmd.extend(["-l", selector])

    payload = run_kubectl(cmd)
    cpu_total = Decimal(0)
    memory_total = Decimal(0)
    pod_count = 0

    for pod in payload.get("items", []):
        namespace = pod.get("metadata", {}).get("namespace")
        if not namespace_allowed(namespace, namespaces):
            continue
        phase = pod.get("status", {}).get("phase")
        if not include_completed and phase in {"Succeeded", "Failed"}:
            continue
        spec = pod.get("spec", {})
        cpu, memory = pod_effective_requests(spec)
        cpu_total += cpu
        memory_total += memory
        pod_count += 1

    return cpu_total, memory_total, pod_count


def original_totals(
    context: Optional[str],
    manifests: List[str],
    namespaces: Optional[Set[str]],
) -> Tuple[Decimal, Decimal, List[str]]:
    cmd = ["create", "--dry-run=client", "-o", "json"]
    if context:
        cmd = ["--context", context, *cmd]
    for manifest in manifests:
        cmd.extend(["-f", manifest])

    payload = run_kubectl(cmd)
    cpu_total = Decimal(0)
    memory_total = Decimal(0)
    skipped = []

    for resource in normalize_items(payload):
        namespace = resource.get("metadata", {}).get("namespace")
        if not namespace_allowed(namespace, namespaces):
            continue
        spec = pod_template_spec(resource)
        if spec is None:
            continue
        replicas = manifest_replicas(resource)
        if replicas is None:
            name = resource.get("metadata", {}).get("name", "<unknown>")
            kind = resource.get("kind", "<unknown>")
            skipped.append(f"{kind}/{name}")
            continue
        cpu, memory = pod_effective_requests(spec)
        cpu_total += cpu * replicas
        memory_total += memory * replicas

    return cpu_total, memory_total, skipped


def original_totals_from_cluster(
    context: Optional[str],
    namespaces: Optional[Set[str]],
    selector: Optional[str],
) -> Tuple[Decimal, Decimal, List[str], int]:
    cmd = ["get", ",".join(TOP_LEVEL_WORKLOAD_KINDS), "-A", "-o", "json"]
    if context:
        cmd = ["--context", context, *cmd]
    if selector:
        cmd.extend(["-l", selector])

    payload = run_kubectl(cmd)
    cpu_total = Decimal(0)
    memory_total = Decimal(0)
    counted = 0
    skipped = []

    for resource in payload.get("items", []):
        namespace = resource.get("metadata", {}).get("namespace")
        if not namespace_allowed(namespace, namespaces):
            continue
        spec = pod_template_spec(resource)
        if spec is None:
            continue
        replicas = cluster_workload_replicas(resource)
        if replicas is None:
            name = resource.get("metadata", {}).get("name", "<unknown>")
            kind = resource.get("kind", "<unknown>")
            skipped.append(f"{kind}/{name}")
            continue
        if replicas == 0:
            continue
        cpu, memory = pod_effective_requests(spec)
        cpu_total += cpu * replicas
        memory_total += memory * replicas
        counted += 1

    return cpu_total, memory_total, skipped, counted


def non_workload_pod_totals(
    context: Optional[str],
    namespaces: Optional[Set[str]],
    selector: Optional[str],
    include_completed: bool,
) -> Tuple[Decimal, Decimal, int]:
    resources = fetch_workload_objects(context, selector)
    resource_index = {resource_key(resource): resource for resource in resources}
    cpu_total = Decimal(0)
    memory_total = Decimal(0)
    pod_count = 0

    for resource in resources:
        if resource.get("kind") != "Pod":
            continue
        namespace = resource.get("metadata", {}).get("namespace")
        if not namespace_allowed(namespace, namespaces):
            continue
        phase = resource.get("status", {}).get("phase")
        if not include_completed and phase in {"Succeeded", "Failed"}:
            continue
        if resolve_workload_kind(resource, resource_index) is not None:
            continue
        cpu, memory = pod_effective_requests(resource.get("spec", {}))
        cpu_total += cpu
        memory_total += memory
        pod_count += 1

    return cpu_total, memory_total, pod_count


def fmt_decimal(value: Decimal, quant: str) -> str:
    return str(value.quantize(Decimal(quant), rounding=ROUND_HALF_UP).normalize())


def fmt_memory_mb(bytes_value: Decimal) -> str:
    mb = bytes_value / (Decimal(1000) ** 2)
    return fmt_decimal(mb, "0.01")


def fmt_memory_gb(bytes_value: Decimal) -> str:
    gb = bytes_value / (Decimal(1000) ** 3)
    return fmt_decimal(gb, "0.001")


def fmt_percent(value: Decimal, total: Decimal) -> str:
    if total == 0:
        return "n/a"
    pct = (value * Decimal(100)) / total
    return f"{fmt_decimal(pct, '0.01')}%"


def print_resource_block(
    title: str,
    cpu_m: Decimal,
    memory_b: Decimal,
    alloc_cpu_m: Decimal,
    alloc_memory_b: Decimal,
) -> None:
    print(title)
    print(
        f"  CPU:    {fmt_decimal(cpu_m, '0.001')}m "
        f"({fmt_decimal(cpu_m / CPU_SCALE, '0.001')} cores, "
        f"{fmt_percent(cpu_m, alloc_cpu_m)} of allocatable)"
    )
    print(
        f"  Memory: {fmt_memory_mb(memory_b)} MB "
        f"({fmt_memory_gb(memory_b)} GB, "
        f"{int(memory_b)} bytes, {fmt_percent(memory_b, alloc_memory_b)} of allocatable)"
    )


def main() -> None:
    args = parse_args()
    namespaces = set(args.namespaces) if args.namespaces else None
    manifests = [str(Path(path)) for path in args.manifests or []]

    if args.original_source == "manifests" and not manifests:
        sys.stderr.write("--manifest is required when --original-source=manifests\n")
        raise SystemExit(2)

    current_cpu_m, current_memory_b, pod_count = current_totals(
        context=args.context,
        namespaces=namespaces,
        selector=args.selector,
        include_completed=args.include_completed,
    )
    non_workload_cpu_m, non_workload_memory_b, non_workload_pod_count = non_workload_pod_totals(
        context=args.context,
        namespaces=namespaces,
        selector=args.selector,
        include_completed=args.include_completed,
    )
    alloc_cpu_m, alloc_memory_b, node_count = allocatable_totals(args.context)
    if args.original_source == "cluster":
        original_cpu_m, original_memory_b, skipped, workload_count = original_totals_from_cluster(
            context=args.context,
            namespaces=namespaces,
            selector=args.selector,
        )
        original_label = "Original requested totals from cluster workload specs"
    else:
        original_cpu_m, original_memory_b, skipped = original_totals(
            context=args.context,
            manifests=manifests,
            namespaces=namespaces,
        )
        workload_count = None
        original_label = "Original requested totals from manifests"

    print("Cluster allocatable totals")
    print(f"  Nodes counted: {node_count}")
    print(f"  CPU:    {fmt_decimal(alloc_cpu_m, '0.001')}m ({fmt_decimal(alloc_cpu_m / CPU_SCALE, '0.001')} cores)")
    print(
        f"  Memory: {fmt_memory_mb(alloc_memory_b)} MB "
        f"({fmt_memory_gb(alloc_memory_b)} GB, {int(alloc_memory_b)} bytes)"
    )
    print()
    print("Current requested totals across live pods")
    print(f"  Pods counted: {pod_count}")
    print(
        f"  CPU:    {fmt_decimal(current_cpu_m, '0.001')}m "
        f"({fmt_decimal(current_cpu_m / CPU_SCALE, '0.001')} cores, "
        f"{fmt_percent(current_cpu_m, alloc_cpu_m)} of allocatable)"
    )
    print(
        f"  Memory: {fmt_memory_mb(current_memory_b)} MB "
        f"({fmt_memory_gb(current_memory_b)} GB, "
        f"{int(current_memory_b)} bytes, {fmt_percent(current_memory_b, alloc_memory_b)} of allocatable)"
    )
    print()
    print("Requested totals across live pods not part of a recognized workload")
    print(f"  Pods counted: {non_workload_pod_count}")
    print(
        f"  CPU:    {fmt_decimal(non_workload_cpu_m, '0.001')}m "
        f"({fmt_decimal(non_workload_cpu_m / CPU_SCALE, '0.001')} cores, "
        f"{fmt_percent(non_workload_cpu_m, alloc_cpu_m)} of allocatable)"
    )
    print(
        f"  Memory: {fmt_memory_mb(non_workload_memory_b)} MB "
        f"({fmt_memory_gb(non_workload_memory_b)} GB, "
        f"{int(non_workload_memory_b)} bytes, {fmt_percent(non_workload_memory_b, alloc_memory_b)} of allocatable)"
    )
    print()
    print(original_label)
    if workload_count is not None:
        print(f"  Workloads counted: {workload_count}")
    print(
        f"  CPU:    {fmt_decimal(original_cpu_m, '0.001')}m "
        f"({fmt_decimal(original_cpu_m / CPU_SCALE, '0.001')} cores, "
        f"{fmt_percent(original_cpu_m, alloc_cpu_m)} of allocatable)"
    )
    print(
        f"  Memory: {fmt_memory_mb(original_memory_b)} MB "
        f"({fmt_memory_gb(original_memory_b)} GB, "
        f"{int(original_memory_b)} bytes, {fmt_percent(original_memory_b, alloc_memory_b)} of allocatable)"
    )
    if skipped:
        print()
        print("Skipped objects")
        for item in skipped:
            print(f"  {item}")


if __name__ == "__main__":
    main()
