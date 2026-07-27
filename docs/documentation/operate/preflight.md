---
icon: lucide/shield-check
title: "Preflight / Health Check"
description: "How CruiseKube's preflight check verifies a cluster is ready — Prometheus connectivity, Kubernetes and node versions, and the metrics the optimizer relies on — and how the dashboard uses it to gate the Overview and generate a shareable report."
keywords:
  - CruiseKube preflight
  - health check
  - readiness
  - Prometheus connectivity
  - Kubernetes version
  - metrics verification
---

# Preflight / Health Check

The **preflight check** verifies that a cluster is correctly set up for CruiseKube before the dashboard shows any data. When a user opens the dashboard, the frontend calls the preflight endpoint; the **Overview page is shown only if every required check passes**. Otherwise the dashboard shows a **status screen** listing exactly what is missing or misconfigured and why.

This gives you a single, authoritative answer to *"is my cluster ready for CruiseKube?"* — and a downloadable report you can share with a teammate or with support.

## Endpoint

```
GET /api/v1/clusters/{clusterID}/preflight
```

- **Auth:** HTTP Basic (same credentials as the rest of the dashboard API — see [Login & authentication](authentication.md)).
- **Cluster ID:** the active cluster; `default` in single-cluster mode.
- **Parameters:** none. All thresholds are **backend policy** and are not client-configurable.
- **Status codes:**
    - `200 OK` — returned whenever the cluster exists, **even when the cluster is not ready**. "Not ready" is a normal result carried in the body (`healthy: false`), not an HTTP error.
    - `404 Not Found` — unknown cluster.
    - `5xx` / network error — the check could not be run.

!!! tip "Quick check from the CLI"
    ```bash
    curl -s -u <user>:<pass> \
      http://localhost:8080/api/v1/clusters/default/preflight \
      | jq '{healthy, summary, failures}'
    ```

## What it checks

The preflight runs three ordered steps and returns a single `healthy` verdict plus a flat `failures` list and full per-check detail.

### 1. Prometheus connectivity & health

CruiseKube confirms it can actually reach the configured Prometheus:

- **Primary probe:** the Prometheus [buildinfo API](https://prometheus.io/docs/prometheus/latest/querying/api/#build-information) (`/api/v1/status/buildinfo`), which also yields the Prometheus version.
- **Fallback probe:** a trivial `vector(1)` instant query, for Prometheus-compatible backends (Thanos, Cortex, Mimir, VictoriaMetrics) that do not implement buildinfo.

If Prometheus cannot be reached, the response reports the exact **target** it tried — the full `url` plus parsed `host` and `port` — so you can diagnose the endpoint. The bearer token is never included.

### 2. Versions

| Component | Minimum | Why |
|-----------|---------|-----|
| Kubernetes **server** (control plane) | **v1.33.0** | Required for pod in-place resource updates (beta / on by default from 1.33) |
| **Node** kubelet (checked per node) | **v1.33.0** | Same feature runs on the nodes; kubelets must also be 1.33+ |
| Prometheus | **2.30.0** | Query features CruiseKube relies on |

!!! info "Why both Kubernetes and node versions?"
    The **Kubernetes version** is the control-plane / API-server version (one per cluster). The **node version** is the kubelet version on each individual node. CruiseKube's core feature — **pod in-place resource updates** (in-place vertical scaling) — is executed by the kubelet on each node. Kubernetes' version-skew policy lets kubelets lag the API server by up to three minor versions, so a cluster can have a 1.33 control plane but older nodes. Checking only the server version would let such a cluster pass while the feature silently does not work on the older nodes — so preflight requires **both** the server and every node's kubelet to be ≥ 1.33. (**PSI (pressure stall information) metrics** are supported but optional — their absence does not block the dashboard.)

The versions step also reports the running **CruiseKube version** (informational — it does not affect the verdict) so it appears in the report.

### 3. Metrics

CruiseKube verifies that the Prometheus metrics it relies on actually exist, grouped by scrape source:

| Group | Job matcher | Examples |
|-------|-------------|----------|
| `kube-state-metrics` | `job="kube-state-metrics"` | `kube_pod_info`, `kube_pod_status_phase`, `kube_node_status_allocatable`, `kube_pod_container_status_restarts_total`, `kube_node_labels` |
| `cadvisor-kubelet` | `job=~"kubelet\|kubernetes-nodes-cadvisor"` | `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes` |
| `node-exporter` | `job="node-exporter"` | `node_load1`, `node_cpu_seconds_total` |
| `psi` (optional) | (kubelet / node-exporter) | `container_pressure_cpu_waiting_seconds_total`, `node_pressure_cpu_waiting_seconds_total`, … |
| `karpenter` | — | `karpenter_nodeclaims_disrupted_total` |

Some metrics are **probed but not required** because they are legitimately absent on a healthy cluster and must not block the dashboard:

- The whole **`psi`** group — PSI (pressure stall information) is kernel/PSI-gated and not enabled on every cluster. PSI metrics are reported (present or not, with labels) but never block the dashboard.
- `kube_node_spec_taint` — no series when no nodes are tainted.
- `kube_pod_container_status_last_terminated_reason` — no series until a container has terminated (e.g. OOM).
- `karpenter_nodeclaims_disrupted_total` — Karpenter-only; empty until a disruption occurs.

**Every probed metric is reported**, whether found or not, along with its **distinct label names** (the dimensions each metric exposes), so the report can show a complete inventory.

## How it works

- **Metric presence** is detected with an instant query of the form `count(last_over_time(<metric>{<job>}[15m]))`. A non-empty result means the metric is present; the 15-minute lookback tolerates counter resets and slow scrape intervals.
- **Distinct labels** for each present metric come from the Prometheus `labels` API (with the internal `__name__` label stripped).
- Metric probes run **concurrently** with a bounded worker pool.
- The whole preflight is bounded by a short **timeout** so a slow or unreachable Prometheus cannot stall the dashboard's first paint.
- If Prometheus is unreachable, the Kubernetes/node version checks still run (they use the Kubernetes API, not Prometheus); the Prometheus version and all metric checks are reported as failed with the connectivity error.

### The `healthy` verdict

`healthy` is `true` only when **all** of the following hold:

- Prometheus is connected and healthy, **and**
- the Kubernetes server version meets the minimum, **and**
- every node's kubelet version meets the minimum, **and**
- the Prometheus version meets the minimum, **and**
- every **required** metric group is present.

Failed **optional** checks are reported but do not affect `healthy`.

### Response shape

```jsonc
{
  "cluster_id": "default",
  "healthy": false,
  "generated_at": "2026-07-08T10:00:00Z",
  "summary": { "total_checks": 24, "passed": 21, "failed": 3 },
  "failures": [
    { "step": "prometheus_connectivity", "item": "prometheus",
      "message": "failed to reach Prometheus at http://prometheus:9090: dial tcp: i/o timeout" },
    { "step": "versions", "item": "kubernetes",
      "message": "kubernetes server version v1.30.0 is below minimum 1.33.0" }
  ],
  "prometheus_connectivity": {
    "connected": true, "healthy": true,
    "target": "http://prometheus:9090", "url": "http://prometheus:9090",
    "host": "prometheus", "port": "9090",
    "probe": "buildinfo", "version": "2.45.0", "revision": "abc123", "error": ""
  },
  "versions": {
    "passed": true,
    "cruisekube_version": "0.3.3",
    "min_kube_version": "1.33.0",
    "min_kubernetes_version": "1.33.0",
    "min_prometheus_version": "2.30.0",
    "kubernetes": { "version": "v1.34.2", "meets_minimum": true },
    "nodes": [ { "name": "node-a", "kubelet_version": "v1.34.2", "meets_minimum": true } ],
    "node_count": 1, "nodes_below_minimum": 0,
    "prometheus": { "version": "2.45.0", "meets_minimum": true }
  },
  "metrics": {
    "passed": true, "lookback": "15m",
    "groups": [
      { "name": "kube-state-metrics", "required": true, "present": true,
        "checks": [
          { "metric": "kube_pod_info", "required": true, "present": true,
            "series": 42, "labels": ["namespace", "node", "pod", "uid"] }
        ] }
    ]
  }
}
```

The flat `failures` array is the quickest thing to render — each entry has a `step`, the specific `item` (a node name, a metric name, `kubernetes`, or `prometheus`), and a human-readable `message`. The nested objects provide the full detail.

## Report generation

The dashboard turns the preflight response into a **downloadable, shareable report**. The report contains **everything** in the response so that a reader with only the report — and no cluster access — can understand the setup:

- The **CruiseKube version** and the timestamp the checks ran.
- The overall verdict and the summary counts.
- The complete list of failures.
- The **Prometheus connectivity** target (URL / host / port), probe used, and detected version.
- Every **version** check: Kubernetes server, each node's kubelet, and Prometheus, each against its minimum.
- Every **metric** check — found or not — including its **distinct labels**.

This makes the report a self-contained artifact for filing issues or handing setup status to a teammate.

## How the dashboard uses it

1. On dashboard open, the frontend calls the preflight endpoint for the active cluster.
2. While it runs, a "Running setup checks…" state is shown.
3. If `healthy` is `true`, the **Overview** page renders normally.
4. If `healthy` is `false`, the **status screen** renders — grouped into Prometheus connectivity, Versions, and Metrics — with a **Retry** action and the option to download the report.

## Troubleshooting

If the preflight reports problems, see [Troubleshooting](operate-troubleshooting.md). Common cases:

- **Prometheus not reachable** — check the reported `target` URL/host/port and that the configured `dependencies.*.prometheusURL` points at a reachable Prometheus.
- **Metrics missing** — confirm kube-state-metrics, cAdvisor/kubelet, and node-exporter are being scraped by the Prometheus CruiseKube is pointed at.
- **Version below minimum** — upgrade the affected node(s) or control plane to Kubernetes 1.33+.
