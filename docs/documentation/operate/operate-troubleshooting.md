---
icon: lucide/life-buoy
title: "Troubleshooting"
description: "Common CruiseKube installation and runtime issues: Prometheus connectivity, webhook failures, empty recommendations, Recommend vs Cruise confusion, and HPA exclusions."
keywords:
  - CruiseKube troubleshooting
  - webhook debug
  - Prometheus
---

# Troubleshooting

Use this page as a **first pass** before opening an issue. Symptom → likely cause → what to check.

---

## No recommendations in the dashboard

| Check | Action |
|-------|--------|
| **Time** | Stats tasks run on a schedule (defaults in `values.yaml`). Wait several intervals after install. |
| **Provider URL** | From a controller pod, the configured metrics provider URL must resolve and be reachable. Use in-cluster Service DNS for Prometheus or the correct remote HTTPS base URL for Kloudfuse. |
| **Metrics** | Confirm the Prometheus-compatible endpoint exposes the required kubelet/cAdvisor, kube-state-metrics, and node-exporter metrics in [Prometheus metric requirements](../reference/prometheus-metrics.md). |
| **Labels and jobs** | Empty results are often caused by `job` values other than `kubelet`, `kube-state-metrics`, or `node-exporter`, or by missing `namespace`, `pod`, `container`, `node`, `created_by_kind`, or `created_by_name` labels. |
| **New workloads** | Very new workloads may be ignored until they pass **`newWorkloadThresholdHours`** (env: `CRUISEKUBE_RECOMMENDATIONSETTINGS_NEWWORKLOADTHRESHOLDHOURS`). |

```bash
kubectl logs -n cruisekube-system deploy/cruisekube-controller-manager --tail=200
```

Look for metrics-provider URL errors, Kloudfuse/Prometheus query errors, `401`/`403` auth failures, TLS issues, `404` missing endpoint errors, or namespace query logs that complete with `container_workloads=0`.

For Kloudfuse specifically, run the validation queries from [Prometheus metric requirements](../reference/prometheus-metrics.md#quick-validation-queries) in the same tenant/project that CruiseKube uses. If raw metrics exist but CruiseKube queries are empty, check whether Kloudfuse ingestion renamed `job`, `node`, or workload-owner labels; normalize those labels at ingestion or with recording rules.

---

## Metrics provider URL and token errors

| Symptom | Checks |
|---------|--------|
| `connection refused`, `no such host`, or timeouts | Exec into the controller pod and verify DNS/network access to the configured `metricsProvider.url`. For in-cluster Prometheus, do not use `localhost`; use the Service DNS name. |
| `401 Unauthorized` / `403 Forbidden` from Kloudfuse | Confirm the bearer token is present in the controller pod env (`CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_BEARERTOKEN`) and has permission to query the target tenant/project. Do not print token values in logs or support tickets. |
| Helm install renders a token inline | Use `cruisekubeController.metricsProvider.bearerTokenExistingSecret` and `bearerTokenExistingSecretKey` instead of `bearerToken`. Inline tokens in config files, CLI args, and Helm values can leak. |

```bash
kubectl exec -n cruisekube-system deploy/cruisekube-controller-manager -- \
  sh -c 'wget -S -O- "$CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_URL/api/v1/query?query=up" 2>&1 | head -40'
```

For authenticated Kloudfuse endpoints, use a temporary local shell variable or Kubernetes Secret reference for the token; avoid pasting tokens into shared terminals or issue comments.

---

## Metrics provider TLS / HTTPS

If Prometheus or Kloudfuse uses a **private CA** or self-signed cert, TLS verification can fail with `x509: certificate signed by unknown authority` or hostname mismatch errors. Prefer installing the proper CA trust in production. For trusted development/test endpoints only, set the active provider TLS skip flag (`CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_INSECURESKIPTLSVERIFY=true` or `CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_INSECURESKIPTLSVERIFY=true`).

---

## Missing PromQL query endpoints

CruiseKube expects Prometheus-compatible instant and range query APIs under the configured base URL. Verify both endpoints exist for the same tenant/project:

```bash
# Substitute the same base URL and auth context used by CruiseKube.
curl -fsS "$METRICS_PROVIDER_URL/api/v1/query?query=up"
curl -fsS --get "$METRICS_PROVIDER_URL/api/v1/query_range" \
  --data-urlencode 'query=up' \
  --data-urlencode 'start=2024-01-01T00:00:00Z' \
  --data-urlencode 'end=2024-01-01T00:05:00Z' \
  --data-urlencode 'step=60s'
```

If Kloudfuse exposes a different tenant/project path, configure `metricsProvider.url` to the Prometheus-compatible query base URL, not only the UI URL.

---

## Missing Kubernetes metrics or label/job mismatches

- Run the [quick validation queries](../reference/prometheus-metrics.md#quick-validation-queries) in the same provider view CruiseKube uses.
- Confirm kubelet/cAdvisor `container_*`, kube-state-metrics `kube_*`, and node-exporter `node_*` metric families are ingested.
- Confirm `job` labels match CruiseKube's expected values: `kubelet`, `kube-state-metrics`, and `node-exporter`.
- Confirm join labels such as `namespace`, `pod`, `container`, `node`, `created_by_kind`, and `created_by_name` are preserved.
- For remote backends such as Kloudfuse, normalize renamed metrics/labels during ingestion or with recording rules before pointing CruiseKube at the endpoint.

---

## Webhook not mutating pods

| Check | Action |
|-------|--------|
| **`MutatingWebhookConfiguration`** | `kubectl get mutatingwebhookconfiguration` — verify CruiseKube webhook exists and points to the correct service. |
| **Certs** | Chart often ships a **cert-gen** job; check webhook pod logs and APIServer warnings. |
| **`failurePolicy`** | Default is often **Ignore**—failures fail open; pods still create but **unmutated**. |
| **Namespace selectors** | System namespaces may be **excluded** by design. |

```bash
kubectl logs -n cruisekube-system deploy/cruisekube-webhook-server --tail=200
```

---

## “Nothing changes” but I enabled Cruise mode

| Check | Action |
|-------|--------|
| **Recommend vs Cruise** | The workload is still on **Recommend** (observe-only)—only **Cruise** applies changes. Confirm in **Policies & Configuration** ([Dashboard](config-dashboard.md), [Policies & modes](operate-policies.md)). |
| **HPA** | CPU/memory metric HPA targets are **skipped** entirely. |
| **Best-effort pods** | Best-effort QoS classes may be excluded from optimization. |

---

## Unexpected evictions

1. Read [Tradeoffs — pod eviction](../concepts/arch-tradeoffs.md#pod-eviction-can-cause-disruption).  
2. Review **eviction priority** for the workload in the dashboard.  
3. Check node **memory/CPU** pressure—optimizer may be enforcing feasibility.  
4. Inspect controller logs around the timestamp for **eviction** messages.

---

## OOM loops or repeated restarts

1. Confirm **memory application** is not disabled while you expect limits to rise.  
2. Review [OOM handling](../concepts/arch-algorithm.md#oom-handling) — cooldown prevents thrashing; repeated OOM may indicate limit/request still too tight for real spikes.  
3. Validate **JVM** and other runtimes that do not tolerate rapid memory changes.

---

## Frontend cannot reach API

- **Port-forward** the frontend **and** ensure `cruisekubeFrontend.backendURL` (Helm) points at the **controller Service** inside the cluster.  
- Check **basic auth** credentials on the controller HTTP API if enabled (`CRUISEKUBE_SERVER_BASICAUTH_*`).

---

## Database connection errors

- Verify Postgres **host/port/secret** matches `global.postgresql.auth.*` when using external DB.  
- For bundled Postgres, check PVC binding and pod readiness:

```bash
kubectl get pods -n cruisekube-system -l app.kubernetes.io/name=postgresql
```

---

## Still stuck?

Collect and attach:

- CruiseKube **chart version** / **app version** (`helm list -n cruisekube-system`)  
- Redacted **`values.yaml`**  
- Controller and webhook **logs** (last ~500 lines)  
- Whether the Prometheus-compatible endpoint can run the [metric validation queries](../reference/prometheus-metrics.md#quick-validation-queries), especially `container_cpu_usage_seconds_total`, `kube_pod_info`, and `node_load1`

Then open a [GitHub Issue](https://github.com/truefoundry/CruiseKube/issues) or ask on [Discord](https://discord.gg/Dqek4xJa3N).
