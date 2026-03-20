---
title: "Installation"
description: "Install CruiseKube with Helm from the OCI registry, verify pods, access the dashboard, and disable dry-run when you are ready to apply recommendations."
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

!!! tip "Suggested media"
    **Screenshot** of the workload list after first successful stats run—even placeholder data helps readers know they are “done” with install.

---

## Apply recommendations: disable dry-run

By default, **dry-run** prevents unintended mutations. When you are ready for live changes, set the controller **apply** task and **webhook** dry-run flags to **`false`**, and align memory application with your policy.

```bash
helm upgrade --install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube --namespace cruisekube-system --create-namespace \
  --set cruisekubeController.env.CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL="http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090" \
  --set postgresql.enabled=true \
  --set cruisekubeController.env.CRUISEKUBE_RECOMMENDATIONSETTINGS_DISABLEMEMORYAPPLICATION=false \
  --set cruisekubeController.env.CRUISEKUBE_CONTROLLER_TASKS_APPLYRECOMMENDATION_METADATA_DRYRUN=false \
  --set cruisekubeWebhook.env.CRUISEKUBE_RECOMMENDATIONSETTINGS_DISABLEMEMORYAPPLICATION=false \
  --set cruisekubeWebhook.env.CRUISEKUBE_WEBHOOK_DRYRUN=false
```

Cross-check keys against the current [`charts/cruisekube/values.yaml`](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml) on the tag you deploy—names are stable but new toggles appear over time.

Read [Policies & modes](dev-cost.md) before flipping enforcement in production.

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
