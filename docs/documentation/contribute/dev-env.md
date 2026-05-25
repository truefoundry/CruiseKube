---
icon: lucide/laptop
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
2. Install Prometheus in the cluster and expose it (e.g. via port-forward), or use Kloudfuse with Prometheus-compatible Kubernetes metrics ingestion.
3. Run CruiseKube with a local config file (e.g. `config.local.yaml`) that points to that metrics provider URL.
4. Optionally run the CruiseKube frontend to view workloads and stats.

---

## 1. Prerequisites

Ensure you have the following installed:

| Tool | Purpose |
|------|---------|
| **Go** 1.24+ | Build and run CruiseKube |
| **Docker** | For Kind (if you use it) |
| **kubectl** | Cluster access |
| **Helm** | Install Prometheus (and optionally CruiseKube in-cluster) |
| **Kind** (optional) | Local Kubernetes cluster |
| **Make** | Convenience targets |

---

## 2. Kubernetes cluster

You can either create a local Kind cluster or use any cluster that your current `kubectl` context points to.

- ### Option A: Kind cluster (recommended for local dev)
Create a Kind cluster (the example config maps Prometheus port 9090 to the host):
```bash
kind create cluster --name cruisekube --config=test/kind-config.yaml
```
The `test/kind-config.yaml` maps host port **9090** to the cluster so that after installing Prometheus you can use `http://localhost:9090` in your config.

- ### Option B: Use current context
If you already have a cluster (minikube, existing cloud cluster, etc.), ensure `kubectl` is pointing to it:
```bash
kubectl config current-context
```
You will need either a reachable Prometheus-compatible endpoint or Prometheus installed in that cluster and **port-forwarded** to `localhost:9090` (or another port and URL in config). See the next section.

---

## 3. Configure a metrics provider

CruiseKube needs one Prometheus-compatible endpoint to fetch metrics. For local development you can either install kube-prometheus-stack (Prometheus only, no Grafana/Alertmanager) and expose it, or point at a Kloudfuse PromQL endpoint that ingests Kubernetes metrics with the expected Prometheus metric names and labels.

### Option A: Install Prometheus via Helm

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

Set **`dependencies.local.prometheusURL`** (or `dependencies.inCluster.prometheusURL` if running in-cluster) in your config to this URL so CruiseKube can reach Prometheus. Alternatively, use the structured `metricsProvider` fields below for Prometheus or Kloudfuse.


### Option B: Use Kloudfuse locally

If your cluster metrics are already ingested into Kloudfuse, configure CruiseKube to use the Kloudfuse Prometheus-compatible query endpoint. Keep the bearer token in an environment variable instead of committing it to `config.local.yaml`:

```bash
export CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_BEARERTOKEN="$KLOUDFUSE_BEARER_TOKEN"
# Optional shorthand env vars can override provider type/url for both local and in-cluster blocks.
export CRUISEKUBE_METRICS_PROVIDER=kloudfuse
export CRUISEKUBE_METRICS_PROVIDER_URL="https://kloudfuse.example.com"
```

Then either rely on the shorthand env vars above, or add a non-secret provider block to `config.local.yaml`:

```yaml
controllerMode: local
dependencies:
  local:
    kubeconfigPath: ""
    prometheusURL: "http://localhost:9090" # legacy fallback; keep for easy Prometheus switching
    metricsProvider:
      type: kloudfuse
      url: "https://kloudfuse.example.com"
      # Do not put real tokens here. Set CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_BEARERTOKEN instead.
      bearerToken: ""
      insecureSkipTLSVerify: false
```

!!! warning "Do not inline bearer tokens"
    Inline bearer tokens in config files or CLI arguments can leak through Git history, shell history, process listings, and copied logs. Use environment variables for local development. For production Helm installs, use an existing Kubernetes Secret rather than `--set ...bearerToken=...`.

---

## 4. Configuration: `config.local.yaml`

Use a local config file so you don’t rely on the default `config.yaml`. A typical choice is `config.local.yaml` (you can copy from `config.yaml` and adjust).

### Run the controller with your config

```bash
go run cmd/cruisekube/main.go --config-file-path config.local.yaml
```

You can override any config value with flags, for example:

- `--config-file-path` — Path to the YAML config (default: `config.yaml`).
- `--controller-mode` — `local` or `in-cluster`; for dev use `local`.
- `--prometheus-url` — Overrides the legacy Prometheus URL in both dependency blocks.
- `--metrics-provider` — Overrides structured provider type (`prometheus` or `kloudfuse`) in both dependency blocks.
- `--metrics-provider-url` — Overrides structured provider URL in both dependency blocks.
- `--metrics-provider-insecure-skip-tls-verify` — Overrides structured provider TLS verification skip in both dependency blocks. Do not pass bearer tokens as CLI args; use env vars.
- `--kubeconfig-path` — Kubeconfig for local mode (empty = use default/current context).
- `--server-port` — HTTP server port (default from config, e.g. 8080).
- `--webhook-port`, `--webhook-certs-dir`, `--webhook-stats-url-host` — Webhook settings.
- `--db-file-path` — SQLite DB path (overrides config).

### Brief description of `config.local.yaml` sections

| Section | Purpose |
|--------|---------|
| **controllerMode** | `local` = run on your machine using kubeconfig; `inCluster` = run inside the cluster. Use `local` for dev. |
| **dependencies.local** | `kubeconfigPath`: path to kubeconfig (empty = current context). `prometheusURL`: legacy Prometheus URL (e.g. `http://localhost:9090`) — **must match your port-forward or Kind port**. `metricsProvider`: optional structured provider (`type`, `url`, `bearerToken`, `insecureSkipTLSVerify`) for Prometheus or Kloudfuse. Put local bearer tokens in `CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_BEARERTOKEN`, not in YAML. |
| **dependencies.inCluster** | Used when `controllerMode` is `inCluster`; set legacy `prometheusURL` to the in-cluster Prometheus service URL, or use `metricsProvider` for structured Prometheus/Kloudfuse configuration. In production Helm installs, source bearer tokens from existing Kubernetes Secrets. |
| **executionMode** | `controller`, `webhook`, or `both`. For local dev you typically use `controller`. |
| **controller.tasks** | Enable/disable and schedule tasks: `createStats`, `fetchMetrics`, `applyRecommendation`, etc. For dev, `createStats` and `fetchMetrics` are usually enabled. |
| **server** | HTTP API port (e.g. 8080), optional `basicAuth` for the stats/API. |
| **webhook** | Webhook port, certs dir, and `statsURL.host` (e.g. `http://localhost:8080`) when the webhook calls back to your local server. |
| **db** | Database: `type: sqlite` with `database: "stats-data/cruisekube.db"` for local dev, or switch to `postgres` and set host/port/credentials. |
| **recommendationSettings** | Thresholds and behavior for recommendations (e.g. `newWorkloadThresholdHours`, `disableMemoryApplication`). |
| **metrics** | Optional metrics server (e.g. port 8081). |
| **telemetry** | Optional OpenTelemetry (tracing). |

For minimal local dev, the critical parts are: **controllerMode: local**, one reachable metrics provider (`dependencies.local.prometheusURL` for legacy Prometheus or `dependencies.local.metricsProvider` for structured Prometheus/Kloudfuse), and **db** pointing to a local SQLite file (e.g. `stats-data/cruisekube.db`).

---

## 5. Run CruiseKube locally

From the repo root:

```bash
go run cmd/cruisekube/main.go --config-file-path config.local.yaml
```

- The process will use the Kubernetes context from your kubeconfig (or `--kubeconfig-path`).
- It will connect to the active Prometheus-compatible metrics provider using the URL in your config/env vars.
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
| **pkg/config/** | Config types and loading: `config.go` (structs), `viper.go` (load/validate), `taskconfig.go` (per-task config). |
| **pkg/adapters/** | **database/** — DB interface and SQLite/Postgres implementations. **kube/** — Kubernetes client. **metricsprovider/prometheus/** — Prometheus client and PromQL. |
| **pkg/cluster/** | Cluster manager and scheduler; coordinates which clusters/namespaces to manage. |
| **pkg/task/** | Scheduled tasks: `taskcreatestats`, `taskfetchmetrics`, `taskapplyrecommendation`, `tasknodeloadmonitoring`, `taskcleanupoomevents`. Helpers in `task/utils/` (metrics, node stats, workload handling). |
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

For Go code quality checks, the repo also exposes:

- `make test` to run the Go test suite.
- `make deadcode` to run the pinned deadcode analyzer with `golang.org/x/tools/cmd/deadcode@v0.43.0 -test ./cmd/cruisekube`.

The CI workflow runs `deadcode` as a separate informational job, so contributors can see current unreachable-function reports without making the pipeline fail.

For testing and contribution process (PRs, changelog, etc.), see the repository-root `CONTRIBUTING.md` and `DEVELOPMENT.md` files.

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
