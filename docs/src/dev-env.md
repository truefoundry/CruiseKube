---
title: "CruiseKube Development Environment"
description: "Set up your development environment for CruiseKube. Learn how to build, test, and contribute to the project with local Kubernetes clusters."
keywords:
  - CruiseKube development
  - development environment setup
  - local development
  - Kubernetes development
  - contributing to CruiseKube
---

# Development Environment

This page describes how to set up and work with the CruiseKube development environment so you can run the controller locally and iterate quickly.

## Overview

For local development you will:

1. Use a Kubernetes cluster (create a Kind cluster or use your current `kubectl` context).
2. Install Prometheus in the cluster and expose it (e.g. via port-forward) so CruiseKube can scrape metrics.
3. Run CruiseKube with a local config file (e.g. `config.local.yaml`) that points to that Prometheus URL.
4. Optionally run the CruiseKube frontend to view workloads and stats.

---

## 1. Prerequisites

Ensure you have the following installed:

| Tool | Purpose |
|------|---------|
| **Go** 1.21+ | Build and run CruiseKube |
| **Docker** | For Kind (if you use it) |
| **kubectl** | Cluster access |
| **Helm** | Install Prometheus (and optionally CruiseKube in-cluster) |
| **Kind** (optional) | Local Kubernetes cluster |
| **Make** | Convenience targets |

---

## 2. Kubernetes cluster

You can either create a local Kind cluster or use any cluster that your current `kubectl` context points to.

### Option A: Kind cluster (recommended for local dev)

Create a Kind cluster (the example config maps Prometheus port 9090 to the host):

```bash
kind create cluster --name cruisekube --config=test/kind-config.yaml
```

The `test/kind-config.yaml` maps host port **9090** to the cluster so that after installing Prometheus you can use `http://localhost:9090` in your config.

### Option B: Use current context

If you already have a cluster (minikube, existing cloud cluster, etc.), ensure `kubectl` is pointing to it:

```bash
kubectl config current-context
```

You will need to install Prometheus in that cluster and then **port-forward** the Prometheus service to `localhost:9090` (or another port and use that URL in config). See the next section.

---

## 3. Install Prometheus and expose it

CruiseKube needs a Prometheus instance to fetch metrics. Install the kube-prometheus-stack (Prometheus only, no Grafana/Alertmanager) and expose it so the URL in your config is reachable.

### Install via Helm

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install prometheus \
        prometheus-community/kube-prometheus-stack \
        --namespace monitoring \
        --create-namespace \
        --set prometheus.service.type=NodePort \
        --set prometheus.service.nodePort=30090 \
        --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
        --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
        --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
        --set grafana.enabled=false \
        --set alertmanager.enabled=false \
        --set kubeStateMetrics.enabled=true \
        --set nodeExporter.enabled=true \
        --set prometheusOperator.enabled=true \
        --wait --timeout=600s
```

- **If you use Kind** with `test/kind-config.yaml`, host port 9090 is already mapped to NodePort 30090, so Prometheus will be at **`http://localhost:9090`**.
- **If you use another cluster**, port-forward the Prometheus service to localhost:

  ```bash
  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
  ```

  Keep this running in a separate terminal. Use **`http://localhost:9090`** in your config (or the host/port you actually use).

Set **`dependencies.local.prometheusURL`** (or `dependencies.inCluster.prometheusURL` if running in-cluster) in your config to this URL so CruiseKube can reach Prometheus.

---

## 4. Configuration: `config.local.yaml`

Use a local config file so you don’t rely on the default `config.yaml`. A typical choice is `config.local.yaml` (you can copy from `config.yaml` and adjust).

### Run the controller with your config

```bash
go run cmd/cruisekube/main.go --config-file-path config.local.yaml
```

You can override any config value with flags, for example:

- `--config-file-path` — Path to the YAML config (default: `config.yaml`).
- `--controller-mode` — `local` or `inCluster`; for dev use `local`.
- `--prometheus-url` — Overrides Prometheus URL from the config.
- `--kubeconfig-path` — Kubeconfig for local mode (empty = use default/current context).
- `--server-port` — HTTP server port (default from config, e.g. 8080).
- `--webhook-port`, `--webhook-certs-dir`, `--webhook-stats-url-host` — Webhook settings.
- `--db-file-path` — SQLite DB path (overrides config).
- `--apply-recommendation-dry-run` — Apply recommendation in dry-run (default: true for safety).

### Brief description of `config.local.yaml` sections

| Section | Purpose |
|--------|---------|
| **controllerMode** | `local` = run on your machine using kubeconfig; `inCluster` = run inside the cluster. Use `local` for dev. |
| **dependencies.local** | `kubeconfigPath`: path to kubeconfig (empty = current context). `prometheusURL`: Prometheus URL (e.g. `http://localhost:9090`) — **must match your port-forward or Kind port**. |
| **dependencies.inCluster** | Used when `controllerMode` is `inCluster`; set `prometheusURL` to the in-cluster Prometheus service URL. |
| **executionMode** | `controller`, `webhook`, or `both`. For local dev you typically use `controller`. |
| **controller.tasks** | Enable/disable and schedule tasks: `createStats`, `fetchMetrics`, `applyRecommendation`, etc. For dev, `createStats` and `fetchMetrics` are usually enabled. |
| **server** | HTTP API port (e.g. 8080), optional `basicAuth` for the stats/API. |
| **webhook** | Webhook port, certs dir, `dryRun`, and `statsURL.host` (e.g. `http://localhost:8080`) when the webhook calls back to your local server. |
| **db** | Database: `type: sqlite` with `database: "stats-data/cruisekube.db"` for local dev, or switch to `postgres` and set host/port/credentials. |
| **recommendationSettings** | Thresholds and behavior for recommendations (e.g. `newWorkloadThresholdHours`, `disableMemoryApplication`). |
| **metrics** | Optional metrics server (e.g. port 8081). |
| **telemetry** | Optional OpenTelemetry (tracing). |

For minimal local dev, the critical parts are: **controllerMode: local**, **dependencies.local.prometheusURL** set to your Prometheus URL, and **db** pointing to a local SQLite file (e.g. `stats-data/cruisekube.db`).

---

## 5. Run CruiseKube locally

From the repo root:

```bash
go run cmd/cruisekube/main.go --config-file-path config.local.yaml
```

- The process will use the Kubernetes context from your kubeconfig (or `--kubeconfig-path`).
- It will connect to Prometheus using the URL in your config.
- The HTTP server (e.g. on 8080) serves stats and APIs used by the webhook and frontend.

---

## 6. SQLite database and browsing stats

When the controller runs with SQLite configured (e.g. `db.database: "stats-data/cruisekube.db"`), it creates the **`stats-data`** directory and the database file there.

You can browse this database with any SQLite client, for example:

- **TablePlus**
- **DB Browser for SQLite**
- CLI: `sqlite3 stats-data/cruisekube.db`

Tables are created and updated as the controller runs its tasks (e.g. `createStats`, `fetchMetrics`). Inspecting them helps with debugging and understanding stored workload stats.

---

## 7. Workload population and frontend

Once the controller is running:

1. **Tasks run on their schedules** (e.g. `fetchMetrics`, `createStats`). Over time, workload stats are written to the DB and exposed via the HTTP API.
2. **After stats are processed**, you can view workloads in the **CruiseKube frontend**.

### Run the CruiseKube frontend

The frontend is in a separate repository. Use it to see workloads and stats served by your local CruiseKube API (e.g. `http://localhost:8080`).

1. Clone and run the frontend (see [cruiseKube-frontend](https://github.com/truefoundry/cruiseKube-frontend)):

   ```bash
   git clone https://github.com/truefoundry/cruiseKube-frontend.git
   cd cruiseKube-frontend
   npm install
   npm run dev
   ```

2. The dev server typically runs on port **3000** with hot reload.
3. Point the frontend to your local CruiseKube backend (e.g. `http://localhost:8080`) as per the frontend repo’s configuration. Once the backend has processed stats, the frontend will show the workloads.

---

## 8. Code structure (for contributors)

If you want to extend or debug CruiseKube, this layout should help you find the right place.

| Path | Purpose |
|------|---------|
| **cmd/cruisekube/main.go** | Entrypoint: flags, config loading (Viper), validation, and starting controller/webhook/server. |
| **pkg/config/** | Config types and loading: `config.go` (structs), `viper.go` (load/validate), `taskConfig.go` (per-task config). |
| **pkg/adapters/** | **database/** — DB interface and SQLite/Postgres implementations. **kube/** — Kubernetes client. **metricsProvider/prometheus/** — Prometheus client and PromQL. |
| **pkg/cluster/** | Cluster manager and scheduler; coordinates which clusters/namespaces to manage. |
| **pkg/task/** | Scheduled tasks: `taskCreateStats`, `taskFetchMetrics`, `taskApplyRecommendation`, `taskModifyEqualCPUResources`, `taskNodeLoadMonitoring`, `taskCleanupOOMEvents`. Helpers in `task/utils/` (metrics, node stats, workload handling). |
| **pkg/server/** | HTTP server and routes for stats, overrides, and APIs used by the webhook and frontend. |
| **pkg/handlers/** | HTTP handlers: workload analysis, overrides, killswitch, recommendation handling, task triggers, UI, webhook admission. |
| **pkg/oom/** | OOM event observation and processing. |
| **pkg/repository/storage/** | Storage layer used by tasks and handlers. |
| **pkg/types/** | Shared types (e.g. workloads, stats). |
| **charts/cruisekube/** | Helm chart for deploying CruiseKube in a cluster. |
| **test/** | Kind config and test utilities. |

Typical development flow:

- Change code in `pkg/` or `cmd/`.
- Run with `go run cmd/cruisekube/main.go --config-file-path config.local.yaml`.
- Use the SQLite DB in `stats-data/` and the frontend to verify behavior.

For testing and contribution process (PRs, changelog, etc.), see [CONTRIBUTING.md](../../CONTRIBUTING.md) and [DEVELOPMENT.md](../../DEVELOPMENT.md).

---

## 9. Alternative: Run CruiseKube in-cluster (Helm)

If you prefer to run CruiseKube inside the cluster instead of on your machine:

1. Build and load the image (for Kind):

   ```bash
   docker build -t cruisekube:latest .
   kind load docker-image cruisekube:latest --name cruisekube
   ```

2. Install with Helm (example; adjust image and Prometheus URL as needed):

   ```bash
   helm upgrade --install cruisekube \
       ./charts/cruisekube \
       --namespace cruisekube \
       --create-namespace \
       --set cruisekubeController.image.repository=cruisekube \
       --set cruisekubeController.image.tag=latest \
       --set cruisekubeController.image.pullPolicy=Never \
       --set cruisekubeController.env.CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL="http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090" \
       --set cruisekubeController.env.CRUISEKUBE_CONTROLLER_TASKS_CREATESTATS_ENABLED=true \
       --set cruisekubeWebhook.image.repository=cruisekube \
       --set cruisekubeWebhook.image.tag=latest \
       --set cruisekubeWebhook.image.pullPolicy=Never \
       --set cruisekubeWebhook.webhook.statsURL.host="http://localhost:8080" \
       --set postgresql.enabled=true \
       --set cruisekubeFrontend.enabled=false \
       --wait --timeout=60s
   ```

3. Redeploy after code changes:

   ```bash
   docker build -t cruisekube:latest .
   kind load docker-image cruisekube:latest --name cruisekube
   kubectl rollout restart deployment/cruisekube-controller -n cruisekube
   kubectl rollout restart deployment/cruisekube-webhook -n cruisekube
   ```

---

## 10. Cleanup

- **Uninstall CruiseKube (Helm):**  
  `helm uninstall cruisekube -n cruisekube`

- **Delete Kind cluster:**  
  `kind delete cluster --name cruisekube`

- **Remove local dev data:**  
  Delete the `stats-data/` directory if you want to start with a fresh SQLite DB.
