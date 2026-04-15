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
| **Prometheus URL** | From a controller pod, the URL must resolve (in-cluster Service DNS, correct namespace). |
| **Metrics** | Confirm Prometheus scrapes **cAdvisor / kubelet / pod** metrics CruiseKube queries. |
| **New workloads** | Very new workloads may be ignored until they pass **`newWorkloadThresholdHours`** (env: `CRUISEKUBE_RECOMMENDATIONSETTINGS_NEWWORKLOADTHRESHOLDHOURS`). |

```bash
kubectl logs -n cruisekube-system deploy/cruisekube-controller-manager --tail=200
```

Look for Prometheus query errors, auth failures, or TLS issues.

---

## Prometheus TLS / HTTPS

If Prometheus uses a **private CA** or self-signed cert, you may need `CRUISEKUBE_DEPENDENCIES_INCLUSTER_INSECURESKIPTLSVERIFY` (or local equivalent) set to `"true"` **only** in trusted environments. Prefer proper CA trust in production. See chart `values.yaml` env keys.

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
- Whether **Prometheus** can run a sample query for `container_cpu_usage_seconds_total`

Then open a [GitHub Issue](https://github.com/truefoundry/CruiseKube/issues) or ask on [Discord](https://discord.gg/Dqek4xJa3N).

!!! tip "Suggested media"
    **Annotated screenshot** of a healthy dashboard + one of `kubectl get pods -n cruisekube-system` for the docs “all green” reference panel.
