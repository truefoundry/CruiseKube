#!/usr/bin/env python3

import argparse
import json
import os
import shutil
import subprocess
import sys
from typing import Dict, List, Optional, Set


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Compare pod names present in the cluster against pod names stored "
            "in the pod_resource_recommendations table."
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
        key = key.strip()
        value = value.strip().strip("'\"")
        values[key] = value

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


def fetch_cluster_pods(context: Optional[str], include_completed: bool) -> Set[str]:
    cmd: List[str] = ["kubectl"]
    if context:
        cmd.extend(["--context", context])
    cmd.extend(["get", "pods", "-A", "-o", "json"])

    proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr or proc.stdout)
        raise SystemExit(proc.returncode)

    payload = json.loads(proc.stdout)
    pods: Set[str] = set()
    for item in payload.get("items", []):
        phase = item.get("status", {}).get("phase")
        if not include_completed and phase in {"Succeeded", "Failed"}:
            continue
        name = item.get("metadata", {}).get("name")
        if name:
            pods.add(name)
    return pods


def fetch_db_pods(db_config: Dict[str, str]) -> Set[str]:
    try:
        import psycopg

        conn = psycopg.connect(
            host=db_config["host"],
            port=int(db_config["port"]),
            dbname=db_config["database"],
            user=db_config["username"],
            password=db_config["password"],
            sslmode=db_config["sslmode"],
        )
        with conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT DISTINCT pod
                    FROM pod_resource_recommendations
                    WHERE pod IS NOT NULL AND pod <> ''
                    """
                )
                return {row[0] for row in cur.fetchall() if row[0]}
    except ImportError:
        pass
    except Exception as exc:
        sys.stderr.write(f"failed to query postgres with psycopg: {exc}\n")
        raise SystemExit(1)

    try:
        import psycopg2

        conn = psycopg2.connect(
            host=db_config["host"],
            port=int(db_config["port"]),
            dbname=db_config["database"],
            user=db_config["username"],
            password=db_config["password"],
            sslmode=db_config["sslmode"],
        )
        with conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT DISTINCT pod
                    FROM pod_resource_recommendations
                    WHERE pod IS NOT NULL AND pod <> ''
                    """
                )
                return {row[0] for row in cur.fetchall() if row[0]}
    except ImportError:
        pass
    except Exception as exc:
        sys.stderr.write(f"failed to query postgres with psycopg2: {exc}\n")
        raise SystemExit(1)

    psql_path = shutil.which("psql")
    if psql_path:
        query = (
            "SELECT DISTINCT pod "
            "FROM pod_resource_recommendations "
            "WHERE pod IS NOT NULL AND pod <> '' "
            "ORDER BY pod;"
        )
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
        return {line.strip() for line in proc.stdout.splitlines() if line.strip()}

    sys.stderr.write(
        "missing PostgreSQL client support. Install either 'psycopg' or 'psycopg2', "
        "or install the 'psql' CLI.\n"
    )
    raise SystemExit(1)


def print_section(title: str, values: Set[str]) -> None:
    print(title)
    print(f"  Count: {len(values)}")
    for value in sorted(values):
        print(f"  {value}")
    print()


def main() -> None:
    args = parse_args()
    db_config = read_db_config(args.config)
    cluster_pods = fetch_cluster_pods(args.context, args.include_completed)
    db_pods = fetch_db_pods(db_config)

    only_in_db = db_pods - cluster_pods
    only_in_cluster = cluster_pods - db_pods

    print("Pod comparison summary")
    print(f"  Cluster pods: {len(cluster_pods)}")
    print(f"  DB pods: {len(db_pods)}")
    print()

    print_section("Present in DB, but not in cluster", only_in_db)
    print_section("Present in cluster, but not in DB", only_in_cluster)


if __name__ == "__main__":
    main()
