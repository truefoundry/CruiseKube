---
icon: lucide/activity
title: "Prometheus metric requirements"
description: "PromQL metric, label, and scrape job assumptions required by CruiseKube recommendations, cluster summaries, and exported health metrics."
keywords:
  - CruiseKube Prometheus metrics
  - CruiseKube PromQL
  - Kloudfuse
  - kube-state-metrics
  - kubelet
---

# Prometheus metric requirements

CruiseKube builds recommendations and cluster summaries from PromQL queries. The Prometheus-compatible endpoint configured for CruiseKube, including remote backends such as Kloudfuse, must expose the metric names, labels, and scrape-job labels below.

CruiseKube does not currently rewrite query label names per provider. If your backend stores the same telemetry under different metric names, job names, or labels, normalize it at ingestion, scrape relabeling, recording-rule, or backend compatibility layer before pointing CruiseKube at it.

## Scrape job assumptions

Several queries include explicit `job` matchers. These values must match the series visible to the CruiseKube query endpoint:

| Source | Expected `job` label | Used for |
| ------ | -------------------- | -------- |
| kubelet / cAdvisor | `kubelet` | Container CPU and memory usage, optional container PSI metrics. |
| kube-state-metrics | `kube-state-metrics` | Pod phases, pod ownership metadata, resource requests, node allocatable/capacity, node labels and taints, OOM/restart signals. |
| node-exporter | `node-exporter` | Node CPU, memory, load average, and optional node PSI metrics. |

If Kloudfuse or another remote store uses a different job label, either preserve the Kubernetes scrape job label during ingestion or create equivalent metrics/recording rules with these `job` values.

## Labels CruiseKube joins on

PromQL joins are label-sensitive. These labels must be present with matching values across the corresponding metric families:

| Label | Where it is required | Notes |
| ----- | -------------------- | ----- |
| `namespace` | kubelet/cAdvisor container metrics and kube-state-metrics pod metrics | Used to scope per-namespace recommendation queries. |
| `pod` | kubelet/cAdvisor container metrics and kube-state-metrics pod metrics | Used to join usage to pod phase and owner metadata. |
| `container` | kubelet/cAdvisor container metrics and `kube_pod_container_*` metrics | Empty container names and `POD` sandbox containers are filtered out. |
| `node` | kubelet/cAdvisor, node-exporter, and kube-state-metrics node/pod metrics | Used for usage-to-node and node-load joins. Ensure node-exporter series use the Kubernetes node name, not only `instance` or hostname, or add a relabel/recording rule. |
| `created_by_kind` | `kube_pod_info` | Used as CruiseKube's workload kind before ReplicaSet-to-Deployment normalization. |
| `created_by_name` | `kube_pod_info` | Used as CruiseKube's workload name before ReplicaSet-to-Deployment normalization. |
| `phase` | `kube_pod_status_phase` | Queries select `Running`, exclude terminal/pending phases, or count `Pending` pods depending on the task. |
| `resource` | `kube_pod_container_resource_requests`, `kube_node_status_allocatable`, `kube_node_status_capacity` | Expected values include `cpu`, `memory`, `nvidia_com_gpu`, and in some summary queries `amd_com_gpu`. |
| `reason` | `kube_pod_container_status_last_terminated_reason`, `karpenter_nodeclaims_disrupted_total` | OOM queries require `reason="OOMKilled"`; Karpenter disruption metrics are grouped by reason. |
| `mode` | `node_cpu_seconds_total` | Cluster summary queries use `mode=~"user|system"`. |
| `key` | `kube_node_spec_taint` | Cluster summary allocatable queries look for `key="nvidia.com/gpu"`. |
| `accelerator` | `kube_node_labels` | Cluster summary allocatable queries look for `accelerator="nvidia"`. |

CruiseKube's application-level cluster id is passed as configuration and used on CruiseKube's own exported metrics. The current PromQL queries do not add a `cluster=...` selector. For multi-cluster Prometheus/Kloudfuse endpoints, configure the provider endpoint or backend view so CruiseKube queries are scoped to the intended cluster, or expose compatibility metrics that are already cluster-scoped.

## Required metric families

### Recommendations and time-series prediction

These metrics feed workload CPU/memory recommendations and time-series prediction. Missing owner metadata or join labels usually results in empty recommendation rows even when raw usage exists.

| Metric | Source | Required labels | Purpose |
| ------ | ------ | --------------- | ------- |
| `container_cpu_usage_seconds_total` | kubelet / cAdvisor (`job="kubelet"`) | `namespace`, `pod`, `container`, `node`, `job` | Per-container CPU usage. Queried with `rate(...[1m])` and aggregated by workload/container. |
| `container_memory_working_set_bytes` | kubelet / cAdvisor (`job="kubelet"`) | `namespace`, `pod`, `container`, `node`, `job` | Per-container memory working set. Queried directly for current and historical memory recommendations. |
| `container_pressure_cpu_waiting_seconds_total` | kubelet (`job="kubelet"`) | `namespace`, `pod`, `container`, `node`, `job` | Optional PSI-aware CPU adjustment when PSI is enabled. If absent, run CruiseKube with PSI disabled. |
| `kube_pod_info` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `node`, `created_by_kind`, `created_by_name`, `job` | Maps pods to Kubernetes workload owners used in recommendation keys. |
| `kube_pod_status_phase` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `phase`, `job` | Filters recommendation inputs to running/non-terminal pods and counts replicas. |

### Resource request, capacity, and allocatable summaries

CruiseKube's PromQL currently requires resource **requests** from kube-state-metrics. Current workload limits and requests used while applying changes are also read from the Kubernetes API. Keep kube-state-metrics resource **limits** available with the same label shape where possible so dashboards and future CruiseKube query additions remain compatible.

| Metric | Source | Required labels | Purpose |
| ------ | ------ | --------------- | ------- |
| `kube_pod_container_resource_requests` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `container`, `resource`, and where available `node` | Cluster CPU/memory request summaries and GPU-workload exclusion. Expected `resource` values include `cpu`, `memory`, `nvidia_com_gpu`, `amd_com_gpu`. |
| `kube_pod_container_resource_limits` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `container`, `resource`, and where available `node` | Not currently queried by CruiseKube PromQL, but should be ingested with the same label shape for resource limit visibility and compatibility. |
| `kube_node_status_allocatable` | kube-state-metrics (`job="kube-state-metrics"`) | `node`, `resource`, `job` | Cluster allocatable CPU/memory and GPU-node exclusion. |
| `kube_node_status_capacity` | kube-state-metrics (`job="kube-state-metrics"`) | `node`, `resource`, `job` | Node load normalization by CPU capacity. |
| `kube_node_spec_taint` | kube-state-metrics (`job="kube-state-metrics"`) | `node`, `key`, `job` | GPU-node exclusion in cluster summary allocatable queries. |
| `kube_node_labels` | kube-state-metrics (`job="kube-state-metrics"`) | `node`, `accelerator`, `job` | GPU-node exclusion in cluster summary allocatable queries. |

### Node CPU, memory, and load

| Metric | Source | Required labels | Purpose |
| ------ | ------ | --------------- | ------- |
| `node_load1` | node-exporter (`job="node-exporter"`) | `node`, `job` | Node-load dashboards and node overload tainting. Joined to `kube_node_status_capacity` on `node`. |
| `node_cpu_seconds_total` | node-exporter (`job="node-exporter"`) | `node`, `mode`, `job` | Cluster CPU utilization summary. |
| `node_memory_MemTotal_bytes` | node-exporter (`job="node-exporter"`) | `node`, `job` | Cluster memory utilization summary. |
| `node_memory_MemFree_bytes` | node-exporter (`job="node-exporter"`) | `node`, `job` | Cluster memory utilization summary. |
| `node_memory_Buffers_bytes` | node-exporter (`job="node-exporter"`) | `node`, `job` | Cluster memory utilization summary. |
| `node_memory_Cached_bytes` | node-exporter (`job="node-exporter"`) | `node`, `job` | Cluster memory utilization summary. |
| `node_pressure_cpu_waiting_seconds_total` | node-exporter (`job="node-exporter"`) | `node`, `job` | Optional node CPU PSI metrics exported by CruiseKube's fetch-metrics task. |
| `node_pressure_memory_waiting_seconds_total` | node-exporter (`job="node-exporter"`) | `node`, `job` | Optional node memory PSI metrics exported by CruiseKube's fetch-metrics task. |

### OOM, scheduling, and optional integrations

| Metric | Source | Required labels | Purpose |
| ------ | ------ | --------------- | ------- |
| `kube_pod_container_status_last_terminated_reason` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `container`, `reason`, `job` | OOM event detection; requires `reason="OOMKilled"`. |
| `kube_pod_container_status_restarts_total` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `container`, `job` | OOM event detection via recent restart increases. |
| `kube_pod_status_phase` | kube-state-metrics (`job="kube-state-metrics"`) | `namespace`, `pod`, `phase`, `job` | Unschedulable/pending pod count. |
| `karpenter_nodeclaims_disrupted_total` | Karpenter | `reason` | Optional Karpenter consolidation/disruption telemetry. If absent, only this exported CruiseKube metric remains empty/zero. |

## Kloudfuse ingestion checklist

Before enabling CruiseKube against a Kloudfuse PromQL endpoint, verify:

1. The endpoint supports Prometheus instant and range queries with standard PromQL functions used by CruiseKube, including `rate`, `increase`, `quantile`, `quantile_over_time`, `max_over_time`, `min_over_time`, `round`, `ceil`, vector matching (`on`, `group_left`), and subqueries (`[7d:]`, `[10m0s:1m]`, etc.).
2. The metrics above retain their Prometheus-compatible names, especially cAdvisor/kubelet `container_*`, kube-state-metrics `kube_*`, and node-exporter `node_*` families.
3. `job` labels are normalized to `kubelet`, `kube-state-metrics`, and `node-exporter` where CruiseKube queries include job matchers.
4. Join labels (`namespace`, `pod`, `container`, `node`) match across kubelet, kube-state-metrics, and node-exporter series for the same Kubernetes objects.
5. `kube_pod_info` includes `created_by_kind` and `created_by_name`; otherwise CruiseKube cannot attribute container metrics to workloads.
6. The queried backend view is scoped to the intended cluster, or all queried series expose compatible cluster scoping through the selected provider configuration/backend view.
7. Retention covers CruiseKube's lookback windows: at least 10 minutes for CPU recommendations, 30 minutes for memory p75 recommendations, 7 days for max memory and replica history, and the configured ML prediction lookback if time-series prediction is enabled.

## Quick validation queries

Run these in the same PromQL endpoint and tenant/project that CruiseKube uses. Replace `$namespace` with an application namespace.

```text
count by (job) (container_cpu_usage_seconds_total{job="kubelet", namespace="$namespace", container!~"POD|"})
```

```text
count by (job) (container_memory_working_set_bytes{job="kubelet", namespace="$namespace", container!~"POD|"})
```

```text
count by (job) (kube_pod_info{job="kube-state-metrics", namespace="$namespace", created_by_kind=~".+", created_by_name=~".+"})
```

```text
count by (job, resource) (kube_pod_container_resource_requests{job="kube-state-metrics", namespace="$namespace", container!="", resource=~"cpu|memory"})
```

```text
count by (job) (node_load1{job="node-exporter"})
```

```text
count by (node) (kube_node_status_capacity{job="kube-state-metrics", resource="cpu"})
```

A healthy endpoint should return non-empty vectors for workloads that have been running long enough to accumulate samples. Empty results usually indicate missing ingestion, a job-label mismatch, or a missing join label.
