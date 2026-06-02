---
icon: lucide/rocket
title: "Installation"
description: "Install CruiseKube with Helm from the OCI registry, verify pods, access the dashboard, and roll out Recommend vs Cruise modes when you are ready."
keywords:
  - CruiseKube installation
  - Helm OCI
  - Kubernetes operator
hide:
  - toc
---

# Installation

Complete [Pre-requisites](gs-prerequisites.md) first (Kubernetes version, Prometheus, PostgreSQL, and tooling). The Helm command below assumes you already have a compatible Prometheus URL — see [Prometheus scenarios](gs-prerequisites.md#prometheus) if you are unsure which setup to use.

## With Helm

Install from the **OCI** registry (replace the Prometheus URL with yours):

```bash
helm install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube \
  --namespace cruisekube-system  \
  --create-namespace \
  --set cruisekubeController.env.CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL="http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090" 
```

> Customize any installation with a [`values.yaml`](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml) file.

<br />



# Uninstall

```bash
helm uninstall cruisekube -n cruisekube-system
kubectl delete namespace cruisekube-system
```
