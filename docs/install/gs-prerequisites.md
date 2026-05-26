---
icon: lucide/list-checks
title: "Pre-requisites"
description: "Kubernetes version, Prometheus-compatible metrics, PostgreSQL, and tooling required before installing CruiseKube with Helm."
keywords:
  - CruiseKube prerequisites
  - Kubernetes 1.33
  - Prometheus-compatible metrics
---

# Pre-requisites

Install these **before** [Installation](gs-installation.md). Missing any item usually shows up as empty recommendations, failing health checks, or webhook errors.


## Cluster and tooling

| Requirement | Notes |
|-------------|--------|
| **Kubernetes 1.33+** | In-place pod resource updates are part of the design; older versions are unsupported. PSI-aware optimization requires 1.34+; see metrics backend section below. |
| **kubectl** | Configured for the target cluster context. |
| **Helm 3** | For installing the official chart (OCI registry). |


## Prometheus-compatible metrics backend

CruiseKube reads **container, node, and Kubernetes metadata metrics** from exactly one Prometheus-compatible query endpoint: either Prometheus itself or Kloudfuse with Prometheus-compatible Kubernetes metrics ingestion enabled. See [Prometheus metric requirements](../documentation/reference/prometheus-metrics.md) for the full metric, label, and `job` inventory, including Kloudfuse ingestion notes.

- You may use an **existing** Prometheus, **Kloudfuse** configured to ingest Kubernetes metrics with Prometheus-compatible metric names/labels, or install Prometheus via Helm (given below).
- Set `cruisekubeController.metricsProvider` (or `CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_*` env vars) for `prometheus` or `kloudfuse` to a URL reachable **from the controller pods** (in-cluster Service URL or remote HTTPS URL, not `localhost`).
- Ensure kubelet/cAdvisor, kube-state-metrics, and node-exporter series preserve the expected `job`, `namespace`, `pod`, `container`, `node`, and workload owner labels. For Kloudfuse, validate this in the same tenant/project and backend view that CruiseKube will query.

??? note "Optional: install Prometheus via Helm"
    If you do not already have Prometheus, you can add **kube-prometheus-stack**. A second install can fail if Prometheus already exists in the namespace—reuse the existing instance instead.

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

**PSI (Pressure Stall Indicator):** CruiseKube’s algorithm is built around **PSI-aware CPU** reasoning on clusters that expose the right metrics (Kubernetes 1.34+ PSI story). If PSI is absent, behavior degrades toward usage-only signals—still useful, but not identical to a full PSI deployment. See [Algorithm](../documentation/concepts/arch-algorithm.md).



## PostgreSQL

CruiseKube persists **workload statistics**, **recommendations**, and **per-workload overrides** in a database.

- **Option A:** **Bitnami PostgreSQL subchart** official Helm chart (`postgresql.enabled=true`), is enabled by default.
- **Option B:** Use your own Postgres and set `global.postgresql.auth.*` (host, port, user, password, database) per [Helm chart reference](../documentation/reference/reference-helm-chart.md).



## Network and RBAC

- Controller and webhook must reach **kube-apiserver**, the configured **Prometheus-compatible metrics provider** (Prometheus or Kloudfuse), and **PostgreSQL**.
- The chart installs **RBAC** and **MutatingWebhookConfiguration** resources; ensure your GitOps / policy engines allow them.



## What you do *not* need (for a minimal install)

- Grafana (optional for you; not required by CruiseKube).  
- A separate metrics long-term store, unless you intentionally use a remote Prometheus-compatible backend such as Kloudfuse.


