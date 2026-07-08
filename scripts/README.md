# Scripts

## `cruisekube_diagnostics.py`

A shareable diagnostics bundle that mirrors the controller **preflight** check and packages the result. It runs with a step-by-step terminal UI (showing exactly what it does and which step it is on) and writes a **masked** report to a log file.

It checks, in order:

1. **Prometheus connectivity & version** (build info) against the minimum (2.30.0).
2. **Kubernetes server + per-node kubelet versions** and the Prometheus version, against the minimums (1.34.0 / 1.34.0 / 2.30.0).
3. **Every metric CruiseKube relies on** — present or not — plus each metric's **distinct label names**.
4. Controller logs — **2 hours by default** (1h minimum enforced).
5. **Redact & write**: key identifiers (namespace, node, workload names, IPs) are pseudonymized consistently (e.g. `ns-1`, `node-1`, `workload-1`) and secrets/tokens/JWTs/emails are redacted.

Prometheus is read over a local port-forward (same model as `check_prometheus_metrics.py`):

```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
python3 scripts/cruisekube_diagnostics.py --port 9090
python3 scripts/cruisekube_diagnostics.py --port 9090 --context my-cluster --since 2h
```

| Flag | Description |
| --- | --- |
| `--port`, `-p` | Local port where Prometheus is forwarded (default: `9090`). |
| `--host` | Prometheus host (default: `127.0.0.1`). |
| `--namespace` | Controller namespace (default: `cruisekube-system`). |
| `--selector` | Controller pod selector (default: `app.kubernetes.io/name=controller`). |
| `--since` | Log window; minimum `1h` enforced (default: `2h`). |
| `--context` | kubectl context (default: current context). |
| `--output` | Report file (default: `cruisekube-diagnostics-<ts>.log`). |
| `--no-mask-ips` | Do not mask IP addresses. |
| `--no-pseudonymize` | Do not pseudonymize namespace/node/workload names. |
| `--no-color` | Disable colored output. |

The step UI prints to **stderr**; the masked report is written to the `--output` file. Version thresholds and the metric list mirror the Go preflight (`pkg/handlers`); keep them in sync.

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
