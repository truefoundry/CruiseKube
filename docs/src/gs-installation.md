---
title: "Installation"
description: "Install CruiseKube with Helm from the OCI registry, verify pods, access the dashboard, and roll out Recommend vs Cruise modes when you are ready."
keywords:
  - CruiseKube installation
  - Helm OCI
  - Kubernetes operator
---

# Installation

Complete [Pre-requisites](gs-prerequisites.md) first (Kubernetes **1.33+**, **Helm**, **Prometheus**, **PostgreSQL**).

---

## Install CruiseKube with Helm

Install from the **OCI** registry (replace the Prometheus URL with yours):

```bash
helm install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube --namespace cruisekube-system --create-namespace \
  --set cruisekubeController.env.CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL="http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090" \
  --set postgresql.enabled=true
```

- Use an **in-cluster** Prometheus Service DNS name reachable from `cruisekube-system`.  
- For **external PostgreSQL**, set `postgresql.enabled=false` and configure `global.postgresql.auth.*` — see [Helm chart reference](reference-helm-chart.md).

???+ note "Optional: install Prometheus via Helm"
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

> Customize any installation with a `values.yaml` file. Environment variables under `cruisekubeController.env` and `cruisekubeWebhook.env` map into the application config—see [Configuration](config.md).

---

## Verify installation

```bash
kubectl get pods -n cruisekube-system
```

Expect **`cruisekube-controller-manager-*`**, **`cruisekube-webhook-server-*`**, and (if enabled) **`cruisekube-frontend-*`** in `Running` state.

---

## Access the dashboard

```bash
kubectl port-forward -n cruisekube-system svc/cruisekube-frontend 3000:3000
```

Open **http://localhost:3000**. See [Dashboard](config-dashboard.md) for UI concepts.

---

## Apply recommendations (Recommend vs Cruise)

Optimization is controlled **per workload** in the UI—there is **no separate global “dry-run mode”** toggle.

1. Open **Policies & Configuration** → **CruiseKube mode & priority** (see [Dashboard](config-dashboard.md)).  
2. Leave workloads on **Recommend** while you validate suggestions against metrics and SLOs.  
3. Switch trusted workloads to **Cruise** when you want the controller and webhook to **apply** right-sizing.

Optional Helm knobs (for example **disabling memory application** while tuning CPU) still exist on the controller and webhook—see [`charts/cruisekube/values.yaml`](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml) and [Configuration](config.md). Align changes with [Policies & modes](dev-cost.md) before wide rollout.

---

## Uninstall

```bash
helm uninstall cruisekube -n cruisekube-system
kubectl delete namespace cruisekube-system
```

!!! warning
    Deleting the namespace removes **PostgreSQL data** when you used the bundled chart with persistence—back up first if you need history.

---

## Next steps

- [Dashboard](config-dashboard.md)  
- [Policies & modes](dev-cost.md)  
- [Troubleshooting](operate-troubleshooting.md)  
- [Architecture](arch-introduction.md)
