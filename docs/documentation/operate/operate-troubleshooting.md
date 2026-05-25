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

### Required metric names

CruiseKube expects these series to exist in the store your URL points at. They usually come from **kube-state-metrics** (pod/node resources) and **node-exporter** (node CPU), but the **metric name** must match—`job` and other labels can differ if your queries still resolve.

| Metric | Role |
|--------|------|
| `node_cpu_seconds_total` | Node CPU usage (rate over modes) for capacity and utilization views |
| `kube_pod_container_resource_requests` | Per-container **requests**; must include `resource="cpu"` and `resource="memory"` time series |
| `kube_node_status_allocatable` | Per-node **allocatable** CPU and memory (and related resources) |

Verify each name returns data (run in Prometheus UI or `curl` against `/api/v1/query`):

```promql
node_cpu_seconds_total
kube_pod_container_resource_requests{resource="cpu"}
kube_pod_container_resource_requests{resource="memory"}
kube_node_status_allocatable{resource="cpu"}
kube_node_status_allocatable{resource="memory"}
```

If any query is empty:

1. **Scrape targets** — Ensure kube-state-metrics and node-exporter (or your distro’s equivalent) are scraped into this Prometheus.
2. **Retention** — Series must cover the lookback windows CruiseKube uses (see controller task schedules in chart `values.yaml`).
3. **RBAC / collectors** — kube-state-metrics needs permission to list pods and nodes; a broken KSM install often drops `kube_*` metrics while node metrics remain.
4. **Wrong Prometheus** — Point `CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` at the instance that actually holds these metrics, not a short-retention or federated subset without them.

Additional series (for example `container_cpu_usage_seconds_total` from the kubelet/cAdvisor scrape) are used for optimization logic; missing **only** the four names above commonly blocks stats and recommendations entirely.

See [Prerequisites — Prometheus](../../install/gs-prerequisites.md) for install pointers.

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
