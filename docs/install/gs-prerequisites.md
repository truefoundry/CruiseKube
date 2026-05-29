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
- CruiseKube expects standard metric names with `job="kube-state-metrics"`, `job="node-exporter"`, and kubelet/cAdvisor series. See [Troubleshooting — Prometheus metrics](../documentation/operate/operate-troubleshooting.md).

Choose **one** of the deployment patterns below before installing CruiseKube.

### Option A — Use an existing Prometheus (recommended when monitoring is already installed)

If **kube-prometheus-stack** (or another Prometheus Operator install) already runs in `monitoring` or elsewhere:

1. Leave the CruiseKube chart bundled stack **disabled**: `kube-prometheus-stack.enabled=false`.
2. Point the controller at the existing Prometheus Service URL, for example:
   `http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090`
3. Ensure CruiseKube's ServiceMonitors are selected by that Prometheus (this chart labels them `release: prometheus` by default; widen `serviceMonitorSelector` on your Prometheus if needed).

You do **not** need a second Prometheus or a second node-exporter for CruiseKube.

### Option B — Bundled Prometheus alongside an existing monitoring stack

Enable the optional subchart when you want a **dedicated** Prometheus for CruiseKube in the same namespace as the release (`kube-prometheus-stack.enabled=true`), but another stack already provides cluster metrics.

The chart defaults to deploying a **Prometheus Operator**, **node-exporter**, and **kube-state-metrics** with the bundle. When another stack already runs these:

- **Must disable:** the **Prometheus Operator** (`prometheusOperator.enabled=false`) and **node-exporter** (`nodeExporter.enabled=false`) — leaving them on causes a reconcile loop and a host-port clash respectively.
- **Leave enabled (optional to disable):** **kube-state-metrics** — a duplicate is harmless; disable only to avoid the extra Deployment.

Details:

| Component | When coexisting | Why |
|-----------|-----------------|-----|
| **Prometheus Operator** | **Disable** — `prometheusOperator.enabled=false` | The Prometheus Operator watches `Prometheus`/`ServiceMonitor` CRs **cluster-wide** by default. Running a second operator (especially a different version) makes both operators fight over the **same** StatefulSets — the Prometheus pod is deleted and recreated in a loop, and it can also destabilize your existing Prometheus. Reuse the existing operator instead ("one operator, multiple `Prometheus` CRs"). |
| **node-exporter** | **Disable** — `nodeExporter.enabled=false` | node-exporter uses **hostNetwork** and binds **port 9100 on each node**. Only one DaemonSet can run per node. A second install fails with *"didn't have free ports for the requested pod ports"*. |
| **kube-state-metrics** | **Optional** — safe to leave enabled | kube-state-metrics is a plain Deployment (no `hostNetwork`/`hostPort`), so a second copy runs fine alongside an existing one. Disabling it (`kubeStateMetrics.enabled=false`) only avoids a duplicate Deployment and a little extra scrape load; keeping it on gives the bundled Prometheus its own dedicated instance. |
| **Prometheus** | enabled (subchart default) | New Prometheus instance (a `Prometheus` CR) for CruiseKube; discovers ServiceMonitors cluster-wide (`serviceMonitorSelector: {}`). The existing operator reconciles it into a StatefulSet. |

The bundled Prometheus **pulls** metrics from the existing node-exporter in other namespaces (for example `monitoring`) — you reuse the per-node exporter, not duplicate it. The new `Prometheus` CR is created by Helm and reconciled by the **existing** operator, so you still get a dedicated Prometheus without a second operator.

```bash
helm upgrade --install cruisekube oci://…/cruisekube \
  --namespace cruisekube-system \
  --create-namespace \
  --set kube-prometheus-stack.enabled=true \
  --set kube-prometheus-stack.prometheusOperator.enabled=false \
  --set kube-prometheus-stack.nodeExporter.enabled=false
  # kube-state-metrics is left enabled here (safe); add
  # --set kube-prometheus-stack.kubeStateMetrics.enabled=false to reuse an existing instance instead.
```

`CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` is set automatically when the bundle is enabled; override only if you use a custom Service name or port.

!!! note "Bundled Prometheus is persistent by default"
    CruiseKube needs metric history, so the chart provisions a **persistent PVC** (`20Gi`, `15d` retention) for the bundled Prometheus out of the box, using the cluster's **default StorageClass**. You can tune or override it:

    ```yaml
    kube-prometheus-stack:
      prometheus:
        prometheusSpec:
          retention: 30d
          storageSpec:
            volumeClaimTemplate:
              spec:
                storageClassName: gp3   # set explicitly if the cluster has no default StorageClass
                accessModes: ["ReadWriteOnce"]
                resources:
                  requests:
                    storage: 50Gi
    ```

    If the cluster has **no default StorageClass** and you don't set one, the Prometheus pod stays `Pending` on an unbound PVC. For throwaway tests you can opt out of persistence with `kube-prometheus-stack.prometheus.prometheusSpec.storageSpec=null` (EmptyDir, data lost on restart).

### Option C — Greenfield (no monitoring stack yet)

Either install **kube-prometheus-stack** once (standalone or via the CruiseKube chart). Chart defaults already enable node-exporter and kube-state-metrics when the bundle is on:

```bash
# Standalone in monitoring (example)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set alertmanager.enabled=false \
  --set grafana.enabled=false \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
```

Or enable the CruiseKube bundle (exporters on by default):

```bash
--set kube-prometheus-stack.enabled=true
```

### Conflicts to avoid

| Situation | Symptom | What to do |
|-----------|---------|------------|
| Second **node-exporter** DaemonSet | Pod pending / *free ports* on host 9100 | Set `kube-prometheus-stack.nodeExporter.enabled=false` and reuse the existing exporter (Option B). |
| Second **Prometheus Operator** (cluster-wide) | Prometheus pod **deleted/recreated in a loop** (`Killing` ~10s after start, never Ready); existing Prometheus also flaps | Set `kube-prometheus-stack.prometheusOperator.enabled=false` so the existing operator manages the new `Prometheus` CR (Option B). |
| Second **Prometheus Operator** managing the same CRDs | CRD upgrade fights, duplicate operators | Prefer Option A (one operator). If you need a second Prometheus CR, use one operator and two `Prometheus` CRs, or disable CRD install: `kube-prometheus-stack.crds.enabled=false` when CRDs already exist. |
| Second **Prometheus** in the **same namespace** with the same operator | Overlapping `Prometheus` CRs | Use a different namespace for the CruiseKube release, or use Option A. |
| Controller cannot reach Prometheus | Empty stats / health check failures | Use an in-cluster `http://…svc:9090` URL, not `localhost`. |

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
