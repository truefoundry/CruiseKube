---
icon: lucide/user
title: "Basic Usage"
description: "Reach the UI with kubectl port-forward, promote workloads from Recommend to Cruise when you trust the numbers, and use Helm controller env vars (for example CRUISEKUBE_RECOMMENDATIONSETTINGS_DISABLEMEMORYAPPLICATION) for phased CPU-only or memory-inclusive rollouts."
keywords:
  - CruiseKube dashboard
  - kubectl port-forward
  - Recommend vs Cruise
  - per-workload optimization
  - Helm values
  - memory recommendation application
---

## Verify installation

```bash
kubectl get pods -n cruisekube-system
```

Expect **`cruisekube-controller-manager-*`**, **`cruisekube-webhook-server-*`**, and (if enabled) **`cruisekube-frontend-*`** in `Running` state.

<br />

## Access the dashboard

```bash
kubectl port-forward -n cruisekube-system svc/cruisekube-frontend 3000:3000
```

Open **http://localhost:3000**. See [Dashboard](../documentation/operate/config-dashboard.md) for UI concepts.

<br />

## Apply recommendations (Recommend vs Cruise)

Optimization is controlled **per workload** in the UI—there is **no separate global “dry-run mode”** toggle.


1. Open **Workloads** → **CruiseKube mode & priority** (see [Dashboard](../documentation/operate/config-dashboard.md)).  
2. Leave workloads on **Recommend** while you validate suggestions against metrics and SLOs.  
3. Switch trusted workloads to **Cruise** when you want the controller and webhook to **apply** right-sizing.

CruiseKube doesn't apply recommendations unless cruise mode is enabled. In Cruise mode, Memory recommendation application remains enabled by default. To disable memory recommendation application, set the following disable flags to `true`:

  - cruisekubeController.env.CRUISEKUBE_RECOMMENDATIONSETTINGS_DISABLEMEMORYAPPLICATION=false

```bash
helm upgrade --install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube --namespace cruisekube-system --create-namespace \
--set cruisekubeController.env.CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL="http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090" \
--set cruisekubeController.env.CRUISEKUBE_RECOMMENDATIONSETTINGS_DISABLEMEMORYAPPLICATION=false
```

> You can check all the available environment variables in the [values.yaml file](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml#L88).

</br>

Optional Helm knobs (for example **disabling memory application** while tuning CPU) still exist on the controller and webhook—see [`charts/cruisekube/values.yaml`](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml) and [Configuration](../documentation/reference/config.md). Align changes with [Policies & modes](../documentation/operate/operate-policies.md) before wide rollout.

