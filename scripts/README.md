# Scripts

## `check_prometheus_metrics.py`

Validate that a **port-forwarded** Prometheus has the metric families CruiseKube needs, and summarize whether kube-state-metrics, node-exporter, and kubelet scrape sources exist in the current cluster.

```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
python3 scripts/check_prometheus_metrics.py --port 9090
python3 scripts/check_prometheus_metrics.py --port 9090 --context my-cluster
python3 scripts/check_prometheus_metrics.py --port 9090 --include-optional
```

| Flag | Description |
| --- | --- |
| `--port`, `-p` | **Required.** Local port where Prometheus is forwarded (e.g. `9090`). |
| `--host` | Host for the forward (default: `127.0.0.1`). |
| `--context` | kubectl context (default: current context). |
| `--include-optional` | Also check optional metrics (e.g. Karpenter). |
| `--quiet`, `-q` | Suppress progress logs (stderr). |

Progress logs print to **stderr**; the report prints to **stdout**. The script loads scrape pools and targets from the Prometheus API (same data as `http://localhost:<port>/service-discovery` and `/targets` in the UI).

Exit code `0` when all required metrics are present (any `job` label); `1` when any are missing. Metrics with non-standard `job` labels still pass but are listed as warnings (CruiseKube’s PromQL uses specific job names).

## `clusterrequests.py`

```bash
python3 scripts/clusterrequests.py --context <your-context>
python3 scripts/clusterrequests.py --context <your-context> -n <namespace>
python3 scripts/clusterrequests.py --context <your-context> --selector app=<label>
python3 scripts/clusterrequests.py --context <your-context> --original-source manifests -f <manifest-path>
```

| Flag | Description |
| --- | --- |
| `--context` | Kubernetes context to use. |
| `-n`, `--namespace` | Limit to a namespace. Repeatable. |
| `--selector` | Label selector for pods and workloads. |
| `--include-completed` | Include `Succeeded` and `Failed` pods. |
| `--original-source` | `cluster` or `manifests`. Default: `cluster`. |
| `-f`, `--manifest` | Manifest file or directory. Required with `--original-source manifests`. |

## `poddbmismatch.py`

```bash
python3 scripts/poddbmismatch.py --context <your-context>
python3 scripts/poddbmismatch.py --config config.local.yaml --context <your-context>
python3 scripts/poddbmismatch.py --context <your-context> --include-completed
```

| Flag | Description |
| --- | --- |
| `--config` | Config file path. Default: `config.yaml`. |
| `--context` | Kubernetes context to use. |
| `--include-completed` | Include `Succeeded` and `Failed` pods. |

## `currentreconcile.py`

```bash
python3 scripts/currentreconcile.py --context <your-context>
python3 scripts/currentreconcile.py --config config.local.yaml --context <your-context>
python3 scripts/currentreconcile.py --context <your-context> --include-completed
```

| Flag | Description |
| --- | --- |
| `--config` | Config file path. Default: `config.yaml`. |
| `--context` | Kubernetes context to use. |
| `--include-completed` | Include `Succeeded` and `Failed` pods. |
