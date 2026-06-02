---
icon: lucide/life-buoy
title: "Troubleshooting"
description: "Common CruiseKube installation and runtime issues: Prometheus connectivity, webhook failures, empty recommendations, Recommend vs Cruise confusion, and HPA exclusions."
keywords:
  - CruiseKube troubleshooting
  - webhook debug
  - Prometheus
---

# Troubleshooting

Use this page as a **first pass** before opening an issue. Symptom → likely cause → what to check.

---

## No recommendations in the dashboard

| Check | Action |
|-------|--------|
| **Time** | Stats tasks run on a schedule (defaults in `values.yaml`). Wait several intervals after install. |
| **Prometheus URL** | From a controller pod, the URL must resolve (in-cluster Service DNS, correct namespace). |
| **Metrics** | Confirm Prometheus scrapes **cAdvisor / kubelet / pod** metrics CruiseKube queries. |
| **New workloads** | Very new workloads may be ignored until they pass **`newWorkloadThresholdHours`** (env: `CRUISEKUBE_RECOMMENDATIONSETTINGS_NEWWORKLOADTHRESHOLDHOURS`). |

```bash
kubectl logs -n cruisekube-system deploy/cruisekube-controller-manager --tail=200
```

Look for Prometheus query errors, auth failures, or TLS issues.

---

## Metrics provider (Prometheus / PromQL)

CruiseKube’s metrics provider talks to a **Prometheus-compatible HTTP API** and runs **PromQL** queries on a schedule. If the dashboard stays empty or controller logs show query failures, validate the backend and metric names below.

### PromQL support

Your endpoint must accept **instant and range queries** the same way Prometheus does (`/api/v1/query`, `/api/v1/query_range`). This works with:

- Prometheus
- Grafana Mimir / Cortex / Thanos **query frontends** (when they expose the Prometheus query API)
- Other vendors **only if** they implement compatible PromQL and return the expected metric labels

Managed metrics stacks that expose a **custom query language** or a limited metric catalog (without the kube-state-metrics / node-exporter style names below) are not supported unless you **remote-write** those series into Prometheus with matching names.

Confirm PromQL from a controller pod (replace URL and namespace):

```bash
kubectl exec -n cruisekube-system deploy/cruisekube-controller-manager -- \
  wget -qO- 'http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090/api/v1/query?query=up' | head -c 400
```

A JSON body with `"status":"success"` means the API is reachable; fix DNS, network policy, TLS, or `CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` if not.

### Metric names CruiseKube depends on

CruiseKube’s Prometheus provider runs **PromQL** against the URL you configure. Metric **names** must match what the controller queries; `job`, `namespace`, and other labels are often filtered in code (examples below use `job="kubelet"`, `job="kube-state-metrics"`, and `job="node-exporter"` from **kube-prometheus-stack**—adjust if your scrape config uses different `job` values).

#### Kubelet / cAdvisor (per-container usage and PSI)

| Metric | Used for |
|--------|----------|
| `container_cpu_usage_seconds_total` | Workload CPU usage, cluster CPU utilization export |
| `container_memory_working_set_bytes` | Workload memory usage, cluster memory utilization export |
| `container_pressure_cpu_waiting_seconds_total` | PSI-aware CPU signals; stats task checks whether PSI data exists |
| `container_pressure_memory_waiting_seconds_total` | Container memory pressure (exported controller metrics) |

#### kube-state-metrics (Kubernetes object state)

| Metric | Used for |
|--------|----------|
| `kube_pod_info` | Map pods to owning workloads (with `created_by_kind` / `created_by_name`) |
| `kube_pod_status_phase` | Running / Pending pods; workload discovery and cluster summaries |
| `kube_pod_container_resource_requests` | CPU and memory **requests** (`resource="cpu"` and `resource="memory"`); GPU request series used to exclude GPU workloads where applicable |
| `kube_node_status_allocatable` | Node allocatable CPU, memory, and GPU capacity |
| `kube_node_status_capacity` | Node CPU capacity (e.g. load ratio in node load monitoring) |
| `kube_pod_container_status_last_terminated_reason` | OOM detection (`reason="OOMKilled"`) |
| `kube_pod_container_status_restarts_total` | OOM / restart correlation |
| `kube_node_spec_taint` | Exclude GPU nodes from certain cluster rollups (`key="nvidia.com/gpu"`) |
| `kube_node_labels` | Exclude NVIDIA accelerator nodes from certain cluster rollups |

#### node-exporter (node-level signals)

| Metric | Used for |
|--------|----------|
| `node_cpu_seconds_total` | Cluster CPU utilization rollups (`mode=~"user|system"`) |
| `node_load1` | Node load monitoring and load-based metrics |
| `node_pressure_cpu_waiting_seconds_total` | Node CPU pressure (PSI) |
| `node_pressure_memory_waiting_seconds_total` | Node memory pressure (PSI) |
| `node_memory_MemTotal_bytes` | Cluster memory utilization expression |
| `node_memory_MemFree_bytes` | Same (with `Buffers` and `Cached`) |
| `node_memory_Buffers_bytes` | Same |
| `node_memory_Cached_bytes` | Same |

#### Optional (environment-specific)

| Metric | Used for |
|--------|----------|
| `karpenter_nodeclaims_disrupted_total` | Karpenter consolidation/eviction counter export (only if you run Karpenter and scrape its metrics) |

Spot-check that the core families exist (Prometheus UI or `/api/v1/query`):

```promql
container_cpu_usage_seconds_total{job="kubelet"}
container_memory_working_set_bytes{job="kubelet"}
kube_pod_container_resource_requests{resource="cpu"}
kube_pod_container_resource_requests{resource="memory"}
kube_node_status_allocatable{resource="cpu"}
kube_node_status_allocatable{resource="memory"}
kube_pod_status_phase{phase="Running"}
kube_pod_info
node_cpu_seconds_total{job="node-exporter"}
```

If queries are empty:

1. **Scrape targets** — Ensure kubelet/cAdvisor, kube-state-metrics, and node-exporter (or equivalents) are scraped into **this** Prometheus.
2. **Retention** — Series must cover lookback windows used by stats and recommendation tasks (see chart `values.yaml` task schedules).
3. **RBAC / collectors** — A broken kube-state-metrics install often drops all `kube_*` series while node/container metrics still appear.
4. **Wrong or incompatible Prometheus** — Point `CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` at the store that actually holds these names, not a short-retention, federated, or heavily filtered view. Production Prometheus often drops series via relabeling, recording rules, or remote-write cost controls — see [Using a Dedicated Prometheus for CruiseKube](../../install/gs-prerequisites.md#using-a-dedicated-prometheus-for-cruisekube).

If **even one** core metric family above is missing or mislabeled, recommendations, dashboard rollups, or exported cluster metrics are often incomplete or unreliable. Optional metrics only affect the features listed in their row (for example Karpenter counters without `karpenter_nodeclaims_disrupted_total`).

See [Prerequisites — Prometheus](../../install/gs-prerequisites.md) for install and dedicated-instance guidance.

---

## Prometheus TLS / HTTPS

If Prometheus uses a **private CA** or self-signed cert, you may need `CRUISEKUBE_DEPENDENCIES_INCLUSTER_INSECURESKIPTLSVERIFY` (or local equivalent) set to `"true"` **only** in trusted environments. Prefer proper CA trust in production. See chart `values.yaml` env keys.

---

## Webhook not mutating pods

| Check | Action |
|-------|--------|
| **`MutatingWebhookConfiguration`** | `kubectl get mutatingwebhookconfiguration` — verify CruiseKube webhook exists and points to the correct service. |
| **Certs** | Chart often ships a **cert-gen** job; check webhook pod logs and APIServer warnings. |
| **`failurePolicy`** | Default is often **Ignore**—failures fail open; pods still create but **unmutated**. |
| **Namespace selectors** | System namespaces may be **excluded** by design. |

```bash
kubectl logs -n cruisekube-system deploy/cruisekube-webhook-server --tail=200
```

---

## “Nothing changes” but I enabled Cruise mode

| Check | Action |
|-------|--------|
| **Recommend vs Cruise** | The workload is still on **Recommend** (observe-only)—only **Cruise** applies changes. Confirm in **Policies & Configuration** ([Dashboard](config-dashboard.md), [Policies & modes](operate-policies.md)). |
| **HPA** | CPU/memory metric HPA targets are **skipped** entirely. |
| **Best-effort pods** | Best-effort QoS classes may be excluded from optimization. |

---

## Unexpected evictions

1. Read [Tradeoffs — pod eviction](../concepts/arch-tradeoffs.md#pod-eviction-can-cause-disruption).  
2. Review **eviction priority** for the workload in the dashboard.  
3. Check node **memory/CPU** pressure—optimizer may be enforcing feasibility.  
4. Inspect controller logs around the timestamp for **eviction** messages.

---

## OOM loops or repeated restarts

1. Confirm **memory application** is not disabled while you expect limits to rise.  
2. Review [OOM handling](../concepts/arch-algorithm.md#oom-handling) — cooldown prevents thrashing; repeated OOM may indicate limit/request still too tight for real spikes.  
3. Validate **JVM** and other runtimes that do not tolerate rapid memory changes.

---

## Frontend cannot reach API

- **Port-forward** the frontend **and** ensure `cruisekubeFrontend.backendURL` (Helm) points at the **controller Service** inside the cluster.  
- Check **basic auth** credentials on the controller HTTP API if enabled (`CRUISEKUBE_SERVER_BASICAUTH_*`).

---

## Database connection errors

- Verify Postgres **host/port/secret** matches `global.postgresql.auth.*` when using external DB.  
- For bundled Postgres, check PVC binding and pod readiness:

```bash
kubectl get pods -n cruisekube-system -l app.kubernetes.io/name=postgresql
```

---

## Still stuck?

Collect and attach:

- CruiseKube **chart version** / **app version** (`helm list -n cruisekube-system`)  
- Redacted **`values.yaml`**  
- Controller and webhook **logs** (last ~500 lines)  
- Whether **Prometheus** can run a sample query for `container_cpu_usage_seconds_total`

Then open a [GitHub Issue](https://github.com/truefoundry/CruiseKube/issues) or ask on [Discord](https://discord.gg/Dqek4xJa3N).
