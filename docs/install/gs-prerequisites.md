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

### Option B — Greenfield (no monitoring stack yet)

If nothing monitors the cluster yet, install **kube-prometheus-stack** once — either standalone or via the CruiseKube bundle. Chart defaults already enable node-exporter and kube-state-metrics when the bundle is on, and the single operator owns everything cluster-wide (no conflicts):

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

`CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` is set automatically when the bundle is enabled; override only if you use a custom Service name or port.

### Option C — Bundled Prometheus with its own operator, alongside an existing monitoring stack

Enable the optional subchart when you want a **dedicated** Prometheus for CruiseKube — with its **own Prometheus Operator** that you can version and configure independently (different operator version, args, resources) — in the same namespace as the release (`kube-prometheus-stack.enabled=true`), while another stack already provides cluster metrics.

The golden rule when two operators coexist:

> **Only one operator may *own* (manage the StatefulSet of) a `Prometheus`/`Alertmanager` instance in a given namespace.** Reading `ServiceMonitor`s is safe to share; *owning instances* must not overlap, or both operators fight over the same StatefulSets — the Prometheus pod is deleted and recreated in a loop, and your existing Prometheus can also flap.

Because your existing operator is **cluster-wide**, it currently owns every namespace — so scoping only the new operator is not enough. You must scope **both**.

The chart deploys a **Prometheus Operator**, **Prometheus**, **node-exporter**, and **kube-state-metrics** with the bundle. When another stack already runs these:

| Component | When coexisting | Why |
|-----------|-----------------|-----|
| **Prometheus Operator** | **Keep enabled, but scope it** — `prometheusInstanceNamespaces` = release namespace | A second operator is fine as long as it owns only its own namespace's instances. Scope it (step 1) and scope the existing operator out of this namespace (step 2). |
| **node-exporter** | **Disable** — `nodeExporter.enabled=false` | node-exporter uses **hostNetwork** and binds **port 9100 on each node**. Only one DaemonSet can run per node. A second install fails with *"didn't have free ports for the requested pod ports"*. The bundled Prometheus scrapes the existing exporter instead. |
| **kube-state-metrics** | **Optional** — safe to leave enabled | kube-state-metrics is a plain Deployment (no `hostNetwork`/`hostPort`), so a second copy runs fine alongside an existing one. Disabling it (`kubeStateMetrics.enabled=false`) only avoids a duplicate Deployment and a little extra scrape load; keeping it on gives the bundled Prometheus its own dedicated instance. |
| **Prometheus** | enabled (subchart default) | New Prometheus instance (a `Prometheus` CR) for CruiseKube; discovers ServiceMonitors cluster-wide (`serviceMonitorSelector: {}`) so it still scrapes the existing per-node exporter. Reconciled by this chart's own operator. |

**1. Scope this chart's operator to own only its own namespace.** Keep ServiceMonitor discovery broad so the dedicated Prometheus still scrapes exporters elsewhere:

```yaml
kube-prometheus-stack:
  enabled: true
  crds:
    enabled: false                 # reuse the shared, cluster-scoped CRDs
  prometheusOperator:
    enabled: true                  # keep this operator ON for separate config control
    prometheusInstanceNamespaces:  # own ONLY this namespace's Prometheus CR
      - cruisekube-system
    alertmanagerInstanceNamespaces:
      - cruisekube-system
    admissionWebhooks:
      enabled: false               # don't install a 2nd cluster-wide PrometheusRule webhook
    kubeletService:
      enabled: false               # let the existing operator own the kube-system kubelet Service
  nodeExporter:
    enabled: false                 # reuse the existing per-node exporter (host port 9100)
  kubeStateMetrics:
    enabled: true
  prometheus:
    prometheusSpec:
      # keep discovery broad so this Prometheus still scrapes exporters in other namespaces
      serviceMonitorSelector: {}
      serviceMonitorNamespaceSelector: {}
```

**2. Change the EXISTING (production) operator so it stops owning this namespace.** This edit is **required** and unavoidable while that operator is cluster-wide. Pick one and apply it to the existing `kube-prometheus-stack` release:

```yaml
# Option 1 — exclude just the CruiseKube namespace from the existing operator (simplest)
prometheusOperator:
  denyNamespaces:
    - cruisekube-system

# Option 2 — make the existing operator own an explicit list that omits CruiseKube
prometheusOperator:
  prometheusInstanceNamespaces:
    - monitoring
    # - <other namespaces it already manages>
```

Then `helm upgrade` the existing release for the change to take effect.

`CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL` is set automatically when the bundle is enabled; override only if you use a custom Service name or port.

!!! warning "Cluster-scoped objects are shared between the two operators"
    - **CRDs** (`monitoring.coreos.com`) are cluster-wide and shared. Keep `crds.enabled=false` on the CruiseKube bundle and make sure this operator's version is **compatible with the installed CRDs** (ideally close to the existing operator's version). Don't let two releases both try to upgrade CRDs.
    - **Admission webhook** (`ValidatingWebhookConfiguration` for `PrometheusRule`) is cluster-wide — disable it on the bundled operator (`admissionWebhooks.enabled=false`).
    - **kubelet Service / ClusterRoles** — disable `kubeletService` on the bundled operator so the two don't both reconcile the same `kube-system` Service.

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

### Conflicts to avoid

| Situation | Symptom | What to do |
|-----------|---------|------------|
| Second **node-exporter** DaemonSet | Pod pending / *free ports* on host 9100 | Set `kube-prometheus-stack.nodeExporter.enabled=false` and reuse the existing exporter (Option C). |
| Second **Prometheus Operator** (cluster-wide) | Prometheus pod **deleted/recreated in a loop** (`Killing` ~10s after start, never Ready); existing Prometheus also flaps | Scope both operators to disjoint namespaces — `prometheusInstanceNamespaces` on the bundled operator and `denyNamespaces` on the existing one (Option C). |
| Second **Prometheus Operator** managing the same CRDs | CRD upgrade fights, duplicate operators | When running the bundled operator (Option C), set `kube-prometheus-stack.crds.enabled=false` so it reuses the existing cluster CRDs instead of fighting to upgrade them; keep its version compatible with those CRDs. |
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
