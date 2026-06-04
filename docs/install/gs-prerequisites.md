---
icon: lucide/list-checks
title: "Pre-requisites"
description: "Kubernetes version, Prometheus, PostgreSQL, and tooling required before installing CruiseKube with Helm."
keywords:
  - CruiseKube prerequisites
  - Kubernetes 1.33
  - Prometheus
---

# Pre-requisites

Install these **before** [Installation](gs-installation.md). Missing any item usually shows up as empty recommendations, failing health checks, or webhook errors.


## Cluster and tooling

| Requirement | Notes |
|-------------|--------|
| **Kubernetes 1.33+** | In-place pod resource updates are part of the design; older versions are unsupported. PSI-aware optimization requires 1.34+; see Prometheus section below. |
| **kubectl** | Configured for the target cluster context. |
| **Helm 3** | For installing the official chart (OCI registry). |


## Prometheus

CruiseKube reads **container and node metrics** (usage, throttling, PSI where exposed, etc.) from **Prometheus**.

- Set `CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` (or equivalent) to a URL reachable **from the controller pods** (in-cluster Service URL, not `localhost`).
- CruiseKube expects standard metric names with `job="kube-state-metrics"`, `job="node-exporter"`, and container/kubelet series with `job=~"kubelet|kubernetes-nodes-cadvisor"` (kube-prometheus-stack often labels cAdvisor scrapes `kubernetes-nodes-cadvisor`). See [Troubleshooting — Prometheus metrics](../documentation/operate/operate-troubleshooting.md).

An existing Prometheus installation does **not** automatically mean it is compatible with CruiseKube. Pick the scenario below that matches your cluster.

### Scenario 1 — Use an existing compatible Prometheus

If **kube-prometheus-stack** (or another Prometheus install) already runs in `monitoring` or elsewhere **and** exposes the required metrics without aggressive filtering:

1. Point the controller at the existing Prometheus Service URL, for example:
   `http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090`
2. Ensure CruiseKube's ServiceMonitors are selected by that Prometheus (this chart labels them `release: prometheus` by default; widen `serviceMonitorSelector` on your Prometheus if needed).

You do **not** need a second Prometheus or a second node-exporter when the existing stack already stores the metrics CruiseKube needs.

```yaml
cruisekubeController:
  env:
    CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```

### Scenario 2 — Greenfield (no monitoring stack yet)

If nothing monitors the cluster yet, install **kube-prometheus-stack** once (the CruiseKube chart does not bundle Prometheus):

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set alertmanager.enabled=false \
  --set grafana.enabled=false \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
```

Then set `cruisekubeController.env.CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` to that Prometheus in-cluster Service URL when you install CruiseKube (for example `http://kube-prometheus-stack-prometheus.monitoring.svc:9090` — confirm the Service name with `kubectl get svc -n monitoring`).

!!! tip "Retention and storage"
    CruiseKube needs enough **retention** and **history** to produce good recommendations. For production, configure persistent storage and a retention window that matches your recommendation lookback (for example 15–30 days) on the Prometheus you point CruiseKube at.

### Scenario 3 — Dedicated standalone Prometheus

Use this when you **already run** Prometheus for alerting and dashboards, but that instance is **not** suitable for CruiseKube — for example because of metric relabeling, recording rules, remote-write filtering, partial retention, disabled scrape jobs, or short retention. CruiseKube may then show **no recommendations**, **incomplete recommendations**, or failing health checks even though production monitoring looks healthy. See [Troubleshooting — Prometheus metrics](../documentation/operate/operate-troubleshooting.md).

| Issue | Why CruiseKube suffers |
|-------|-------------------------|
| **Metric relabeling at ingest** | Required series are dropped or renamed before they reach the query API. |
| **Recording rules** | Raw kubelet or cAdvisor metrics are replaced by aggregates CruiseKube does not query. |
| **Remote-write pipelines** | Metrics are forwarded to long-term storage with only a subset retained locally. |
| **Partial retention** | Only a fraction of Kubernetes metrics is kept to control cost. |
| **Disabled scrape jobs** | kubelet, kube-state-metrics, or node-exporter targets are not scraped. |
| **Short retention** | Data ages out before CruiseKube's lookback windows can use it. |

**You do not need to replace your existing monitoring stack.** Deploy a **second Prometheus** in its own namespace, used only by CruiseKube. Prefer the official [Prometheus Helm chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus) with a static scrape config over a second full kube-prometheus-stack — fewer resources, no second Prometheus Operator, and simpler troubleshooting.

The dedicated instance should **scrape** kube-state-metrics, node-exporter, and kubelet (cAdvisor) with standard job names; **store raw metrics** without aggressive drops; **retain** at least ~15 days of history (unless you tune CruiseKube schedules); and **expose** `/api/v1/query` and `/api/v1/query_range` to controller pods on an in-cluster URL.

1. Save the following as `standalone-prometheus-values.yaml`. It disables bundled node-exporter and kube-state-metrics (so you do not conflict with existing DaemonSets) and discovers existing cluster targets via `kubernetes_sd_configs`.

```yaml
serverFiles:
  prometheus.yml:
    scrape_configs:
      - job_name: kube-state-metrics
        kubernetes_sd_configs:
          - role: endpoints
        relabel_configs:
          - source_labels:
              - __meta_kubernetes_service_name
            regex: prometheus-kube-state-metrics
            action: keep

      - job_name: node-exporter
        kubernetes_sd_configs:
          - role: endpoints
        relabel_configs:
          - source_labels:
              - __meta_kubernetes_service_name
            regex: prometheus-prometheus-node-exporter
            action: keep

      - job_name: kubelet
        scheme: https
        kubernetes_sd_configs:
          - role: node
        tls_config:
          insecure_skip_verify: true
        bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

prometheus-node-exporter:
  enabled: false

kube-state-metrics:
  enabled: false

prometheus-pushgateway:
  enabled: false

alertmanager:
  enabled: false
```

2. Install Prometheus:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install cruisekube-prometheus prometheus-community/prometheus \
  --namespace cruisekube-metrics \
  --create-namespace \
  -f standalone-prometheus-values.yaml
```

3. Point CruiseKube at the new Service (adjust name/namespace after `kubectl get svc -n cruisekube-metrics`):

```yaml
cruisekubeController:
  env:
    CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL: "http://cruisekube-prometheus-server.cruisekube-metrics.svc:9090"
```

!!! note "Reusing exporters"
    Reuse the cluster's existing **node-exporter** DaemonSet (only one process can bind host port 9100 per node) and scrape it from the dedicated Prometheus. The scrape config above matches common kube-prometheus-stack Service names (`prometheus-kube-state-metrics`, `prometheus-prometheus-node-exporter`); adjust the relabel regex if your install uses different names. kube-state-metrics can be scraped from an existing Deployment or installed alongside the dedicated Prometheus if policy requires isolation.

**PSI (Pressure Stall Indicator):** CruiseKube's algorithm is built around **PSI-aware CPU** reasoning on clusters that expose the right metrics (Kubernetes 1.34+ PSI story). If PSI is absent, behavior degrades toward usage-only signals—still useful, but not identical to a full PSI deployment. See [Algorithm](../documentation/concepts/arch-algorithm.md).



## PostgreSQL

CruiseKube persists **workload statistics**, **recommendations**, and **per-workload overrides** in a database.

- **Option A:** **Bitnami PostgreSQL subchart** official Helm chart (`postgresql.enabled=true`), is enabled by default.
- **Option B:** Use your own Postgres and set `global.postgresql.auth.*` (host, port, user, password, database) per [Helm chart reference](../documentation/reference/reference-helm-chart.md).


## Network and RBAC

- Controller and webhook must reach **kube-apiserver**, **Prometheus**, and **PostgreSQL**.  
- The chart installs **RBAC** and **MutatingWebhookConfiguration** resources; ensure your GitOps / policy engines allow them.


## What you do *not* need (for a minimal install)

- Grafana (optional for you; not required by CruiseKube).  
- A separate metrics long-term store (CruiseKube queries Prometheus directly).
