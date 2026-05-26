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

## With Helm

Install from the **OCI** registry. For a plain Prometheus endpoint, replace the Prometheus URL with yours:

```bash
helm install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube \
  --namespace cruisekube-system  \
  --create-namespace \
  --set cruisekubeController.metricsProvider.type=prometheus \
  --set cruisekubeController.metricsProvider.url="http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```


### Kloudfuse metrics provider with an existing Secret

For Kloudfuse, create the bearer-token Secret yourself and reference it from Helm. The controller will read the token from the Secret-backed environment variable instead of storing it directly in rendered values.

```bash
kubectl create namespace cruisekube-system --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic kloudfuse-metrics-token \
  --namespace cruisekube-system \
  --from-literal=token="$KLOUDFUSE_BEARER_TOKEN"

helm install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube \
  --namespace cruisekube-system \
  --create-namespace \
  --set cruisekubeController.metricsProvider.type=kloudfuse \
  --set cruisekubeController.metricsProvider.url="https://kloudfuse.example.com" \
  --set cruisekubeController.metricsProvider.bearerTokenExistingSecret=kloudfuse-metrics-token \
  --set cruisekubeController.metricsProvider.bearerTokenExistingSecretKey=token
```

!!! warning "Do not inline bearer tokens"
    Inline bearer tokens in config files, CLI arguments, or Helm values can leak through Git history, shell history, process listings, Helm release metadata, and support bundles. Use environment variables for local development and existing Kubernetes Secrets for production Helm installs.

> Customize any installation with a [`values.yaml`](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml) file.

<br />



# Uninstall

```bash
helm uninstall cruisekube -n cruisekube-system
kubectl delete namespace cruisekube-system
```
