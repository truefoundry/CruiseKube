#!/usr/bin/env python3

import argparse
import json
import os
import shutil
import subprocess
import sys
from decimal import Decimal, InvalidOperation
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
RESOURCE_FIELDS = (
    "cpu_request",
    "memory_request",
    "cpu_limit",
    "memory_limit",
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Reconcile pod_resource_recommendations.current against live pod "
            "container resources in the cluster."
        )
    )
    parser.add_argument(
        "--config",
        default="config.yaml",
        help="Path to the config file. Default: config.yaml",
    )
    parser.add_argument(
        "--context",
        help="Optional kubeconfig context to pass to kubectl.",
    )
    parser.add_argument(
        "--include-completed",
        action="store_true",
        help="Include pods in Succeeded or Failed phase from the cluster.",
    )
    return parser.parse_args()


def read_db_config(config_path: str) -> Dict[str, str]:
    try:
        with open(config_path, "r", encoding="utf-8") as handle:
            lines = handle.readlines()
    except OSError as exc:
        sys.stderr.write(f"failed to read config file {config_path}: {exc}\n")
        raise SystemExit(1)

    in_db_block = False
    db_indent = 0
    values: Dict[str, str] = {}

    for raw_line in lines:
        line = raw_line.rstrip("\n")
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        indent = len(line) - len(line.lstrip(" "))
        if not in_db_block:
            if stripped == "db:":
                in_db_block = True
                db_indent = indent
            continue

        if indent <= db_indent:
            break

        if ":" not in stripped:
            continue

        key, value = stripped.split(":", 1)
        values[key.strip()] = value.strip().strip("'\"")

    if not values:
        sys.stderr.write(f"db block not found in {config_path}\n")
        raise SystemExit(1)

    if values.get("type") != "postgres":
        sys.stderr.write(
            f"db.type must be postgres in {config_path}, found: {values.get('type', '<missing>')}\n"
        )
        raise SystemExit(1)

    return {
        "host": values.get("host", "localhost"),
        "port": values.get("port", "5432"),
        "database": values.get("database", ""),
        "username": values.get("username", ""),
        "password": values.get("password", ""),
        "sslmode": values.get("sslmode", "disable"),
    }


def run_kubectl(args: List[str]) -> Dict:
    proc = subprocess.run(["kubectl", *args], capture_output=True, text=True, check=False)
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


def parse_cpu_millicores(value: Optional[str]) -> Decimal:
    if not value:
        return Decimal(0)
    if value.endswith("m"):
        return Decimal(value[:-1] or "0")
    return Decimal(value) * CPU_SCALE


def parse_memory_bytes(value: Optional[str]) -> Decimal:
    if not value:
        return Decimal(0)
    number, suffix = split_quantity(value)
    if suffix not in MEMORY_FACTORS:
        raise ValueError(f"unsupported memory suffix in quantity: {value}")
    return number * MEMORY_FACTORS[suffix]


def millicores_to_cores(value: Decimal) -> Decimal:
    return value / CPU_SCALE


def bytes_to_mb(value: Decimal) -> Decimal:
    return value / (Decimal(1000) ** 2)


def parse_db_current(value: str) -> Dict[str, Decimal]:
    payload = json.loads(value or "{}", parse_float=Decimal, parse_int=Decimal)
    return {
        "cpu_request": Decimal(payload.get("cpu_request", 0) or 0),
        "memory_request": Decimal(payload.get("memory_request", 0) or 0),
        "cpu_limit": Decimal(payload.get("cpu_limit", 0) or 0),
        "memory_limit": Decimal(payload.get("memory_limit", 0) or 0),
    }


def fetch_cluster_records(
    context: Optional[str],
    include_completed: bool,
) -> Dict[Tuple[str, str, str], Dict[str, Decimal]]:
    cmd = ["get", "pods", "-A", "-o", "json"]
    if context:
        cmd = ["--context", context, *cmd]

    payload = run_kubectl(cmd)
    records: Dict[Tuple[str, str, str], Dict[str, Decimal]] = {}

    for pod in payload.get("items", []):
        phase = pod.get("status", {}).get("phase")
        if not include_completed and phase in {"Succeeded", "Failed"}:
            continue
        namespace = pod.get("metadata", {}).get("namespace", "default")
        pod_name = pod.get("metadata", {}).get("name")
        if not pod_name:
            continue
        for container in pod.get("spec", {}).get("containers", []) or []:
            container_name = container.get("name")
            if not container_name:
                continue
            requests = container.get("resources", {}).get("requests", {})
            limits = container.get("resources", {}).get("limits", {})
            records[(namespace, pod_name, container_name)] = {
                "cpu_request": millicores_to_cores(parse_cpu_millicores(requests.get("cpu"))),
                "memory_request": bytes_to_mb(parse_memory_bytes(requests.get("memory"))),
                "cpu_limit": millicores_to_cores(parse_cpu_millicores(limits.get("cpu"))),
                "memory_limit": bytes_to_mb(parse_memory_bytes(limits.get("memory"))),
            }

    return records


def fetch_db_records_psycopg(db_config: Dict[str, str]) -> Optional[Dict[Tuple[str, str, str], Dict[str, Decimal]]]:
    try:
        import psycopg
    except ImportError:
        return None

    query = """
        SELECT DISTINCT ON (namespace, pod, container)
            namespace,
            pod,
            container,
            current
        FROM pod_resource_recommendations
        WHERE pod IS NOT NULL
          AND pod <> ''
          AND container IS NOT NULL
          AND container <> ''
        ORDER BY namespace, pod, container, updated_at DESC NULLS LAST, id DESC
    """
    try:
        conn = psycopg.connect(
            host=db_config["host"],
            port=int(db_config["port"]),
            dbname=db_config["database"],
            user=db_config["username"],
            password=db_config["password"],
            sslmode=db_config["sslmode"],
        )
        records: Dict[Tuple[str, str, str], Dict[str, Decimal]] = {}
        with conn:
            with conn.cursor() as cur:
                cur.execute(query)
                for namespace, pod, container, current in cur.fetchall():
                    records[(namespace or "default", pod, container)] = parse_db_current(current)
        return records
    except Exception as exc:
        sys.stderr.write(f"failed to query postgres with psycopg: {exc}\n")
        raise SystemExit(1)


def fetch_db_records_psycopg2(db_config: Dict[str, str]) -> Optional[Dict[Tuple[str, str, str], Dict[str, Decimal]]]:
    try:
        import psycopg2
    except ImportError:
        return None

    query = """
        SELECT DISTINCT ON (namespace, pod, container)
            namespace,
            pod,
            container,
            current
        FROM pod_resource_recommendations
        WHERE pod IS NOT NULL
          AND pod <> ''
          AND container IS NOT NULL
          AND container <> ''
        ORDER BY namespace, pod, container, updated_at DESC NULLS LAST, id DESC
    """
    try:
        conn = psycopg2.connect(
            host=db_config["host"],
            port=int(db_config["port"]),
            dbname=db_config["database"],
            user=db_config["username"],
            password=db_config["password"],
            sslmode=db_config["sslmode"],
        )
        records: Dict[Tuple[str, str, str], Dict[str, Decimal]] = {}
        with conn:
            with conn.cursor() as cur:
                cur.execute(query)
                for namespace, pod, container, current in cur.fetchall():
                    records[(namespace or "default", pod, container)] = parse_db_current(current)
        return records
    except Exception as exc:
        sys.stderr.write(f"failed to query postgres with psycopg2: {exc}\n")
        raise SystemExit(1)


def fetch_db_records_psql(db_config: Dict[str, str]) -> Optional[Dict[Tuple[str, str, str], Dict[str, Decimal]]]:
    psql_path = shutil.which("psql")
    if not psql_path:
        return None

    query = """
        SELECT row_to_json(t)
        FROM (
            SELECT DISTINCT ON (namespace, pod, container)
                namespace,
                pod,
                container,
                current
            FROM pod_resource_recommendations
            WHERE pod IS NOT NULL
              AND pod <> ''
              AND container IS NOT NULL
              AND container <> ''
            ORDER BY namespace, pod, container, updated_at DESC NULLS LAST, id DESC
        ) AS t
    """
    cmd = [
        psql_path,
        "-h",
        db_config["host"],
        "-p",
        db_config["port"],
        "-U",
        db_config["username"],
        "-d",
        db_config["database"],
        "-At",
        "-c",
        query,
    ]
    env = dict(os.environ)
    env["PGPASSWORD"] = db_config["password"]
    env["PGSSLMODE"] = db_config["sslmode"]

    proc = subprocess.run(cmd, capture_output=True, text=True, check=False, env=env)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr or proc.stdout)
        raise SystemExit(proc.returncode)

    records: Dict[Tuple[str, str, str], Dict[str, Decimal]] = {}
    for line in proc.stdout.splitlines():
        if not line.strip():
            continue
        row = json.loads(line, parse_float=Decimal, parse_int=Decimal)
        namespace = row.get("namespace") or "default"
        pod = row.get("pod")
        container = row.get("container")
        if not pod or not container:
            continue
        records[(namespace, pod, container)] = parse_db_current(row.get("current") or "{}")
    return records


def fetch_db_records(db_config: Dict[str, str]) -> Dict[Tuple[str, str, str], Dict[str, Decimal]]:
    records = fetch_db_records_psycopg(db_config)
    if records is not None:
        return records

    records = fetch_db_records_psycopg2(db_config)
    if records is not None:
        return records

    records = fetch_db_records_psql(db_config)
    if records is not None:
        return records

    sys.stderr.write(
        "missing PostgreSQL client support. Install either 'psycopg' or 'psycopg2', "
        "or install the 'psql' CLI.\n"
    )
    raise SystemExit(1)


def decimal_str(value: Decimal) -> str:
    return format(value.normalize(), "f")


def diff_fields(db_record: Dict[str, Decimal], cluster_record: Dict[str, Decimal]) -> List[str]:
    mismatches = []
    for field in RESOURCE_FIELDS:
        if db_record.get(field, Decimal(0)) != cluster_record.get(field, Decimal(0)):
            mismatches.append(field)
    return mismatches


def print_missing_section(title: str, values: Set[Tuple[str, str, str]]) -> None:
    print(title)
    print(f"  Count: {len(values)}")
    for namespace, pod, container in sorted(values):
        print(f"  {namespace}/{pod}/{container}")
    print()


def print_mismatch_section(
    title: str,
    values: List[Tuple[Tuple[str, str, str], List[str], Dict[str, Decimal], Dict[str, Decimal]]],
) -> None:
    print(title)
    print(f"  Count: {len(values)}")
    for key, fields, db_record, cluster_record in sorted(values, key=lambda item: item[0]):
        namespace, pod, container = key
        parts = []
        for field in fields:
            parts.append(
                f"{field}=db:{decimal_str(db_record.get(field, Decimal(0)))} "
                f"cluster:{decimal_str(cluster_record.get(field, Decimal(0)))}"
            )
        print(f"  {namespace}/{pod}/{container} -> {', '.join(parts)}")
    print()


def main() -> None:
    args = parse_args()
    db_config = read_db_config(args.config)
    db_records = fetch_db_records(db_config)
    cluster_records = fetch_cluster_records(args.context, args.include_completed)

    db_keys = set(db_records.keys())
    cluster_keys = set(cluster_records.keys())
    only_in_db = db_keys - cluster_keys
    only_in_cluster = cluster_keys - db_keys

    mismatches = []
    for key in db_keys & cluster_keys:
        fields = diff_fields(db_records[key], cluster_records[key])
        if fields:
            mismatches.append((key, fields, db_records[key], cluster_records[key]))

    print("Current resource reconciliation summary")
    print(f"  DB pod/container rows: {len(db_records)}")
    print(f"  Cluster pod/containers: {len(cluster_records)}")
    print()

    print_missing_section("Present in DB, but not in cluster", only_in_db)
    print_missing_section("Present in cluster, but not in DB", only_in_cluster)
    print_mismatch_section("Present in both, but current resources differ", mismatches)


if __name__ == "__main__":
    main()
