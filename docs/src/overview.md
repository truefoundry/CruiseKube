---
title: "Introduction"
description: "CruiseKube is Kubernetes-native continuous resource optimization—right-sizing CPU and memory at admission and runtime without treating every pod like a worst-case island."
keywords:
  - CruiseKube
  - Kubernetes resource optimization
  - pod right-sizing
  - in-place pod resize
  - PSI metrics
---

# Meet CruiseKube

**CruiseKube** is a Kubernetes controller which recommends optimized CPU and memory for workloads. It watches real behavior, learns stable demand and spikes, and applies **in-place** request updates so you stop paying for guesses, without giving up the guardrails that keep services reliable.


## Problem: Why Kubernetes Clusters Stay Expensive

Kubernetes gives you bin-packing (cluster autoscaler, Karpenter, etc.), but **waste often lives at the pod**: identical templates, peak-sized requests, and limits that throttle or kill at the wrong time. CruiseKube closes the loop **per pod, on the node where it actually runs**.

If you have ever bumped requests "just to be safe" run fat nodes because of a few noisy neighbors, or asked a team to manually tune YAML every quarter, CruiseKube is aimed at you.

```mermaid
flowchart TB
  Problems --> over["Over provisioning: 
  <br />Fear of CPU throttling and OOMKills leads teams to over-request resources, creating significant waste."]
  Problems --> mannual["Operational Lag: 
  <br />Manually editing YAML files"]
  Problems --> scaling["Inefficient Scaling: 
  <br />Inflated requests result in underutilized nodes, forcing cluster autoscalers into inefficient and costly decisions."]

```

---

## What CruiseKube does differently

| You get | Why it matters |
|--------|----------------|
| **Admission-time sizing** | New pods start closer to reality before the scheduler commits capacity. |
| **Continuous runtime optimization** | Running pods are adjusted on a schedule—**no rolling restart** for request changes where the cluster supports in-place resize. |
| **Node-aware headroom sharing** | Spike capacity is **shared across pods on a node**, instead of every pod reserving its own private peak. |
| **PSI-informed CPU** | CPU decisions can incorporate **pressure / contention** signals—not just raw usage averages. |
| **Memory with a safety story** | Requests converge toward steady demand; **limits** retain headroom; **OOM handling** feeds learning back into the next admission pass. |
| **Explicit priorities** | When the math does not fit, **eviction order** follows policies you set—not random chaos. |

For a line-by-line comparison to Vertical Pod Autoscaler, see [CruiseKube vs VPA](comp-vs-vpa.md).

---


## Safe adoption path

CruiseKube is built for **progressive trust**:

1. **Install** with Helm, wire **Prometheus** and **PostgreSQL** (or use the bundled chart options).  
2. **Explore** recommendations in the dashboard—workloads start in **Recommend** (observe-only) until you opt in; see [Installation](gs-installation.md) and [Policies & modes](operate-policies.md).  
3. **Enable Cruise mode** per workload when you are ready for applied changes.  
4. **Tune priorities and pricing** so cost views and eviction behavior match your risk model—[Resource pricing](operate-resource-pricing.md) and [Tradeoffs](arch-tradeoffs.md).

```mermaid
flowchart LR
  A[Install] --> B[Dashboard — recommendations]
  B --> C{Comfort level}
  C -->|Conservative| D[Recommend only]
  C -->|Ready| E[Cruise mode + priorities]
  E --> F[Monitor savings & SLOs]
```

!!! tip "Suggested media — product walkthrough"
    A **screen recording GIF** of: port-forward → login → open a workload → flip mode → show before/after request columns or savings panel.

---

## Requirements in brief

- **Kubernetes 1.33+** (in-place pod resource updates are central to the design).  
- **Prometheus** with the metrics CruiseKube expects (see [Pre-requisites](gs-prerequisites.md)).  
- **PostgreSQL** (managed by you or the subchart).

---

## Next steps

| Step | Page |
|------|------|
| Understand scenarios | [Use cases](overview-use-cases.md) |
| Compare to VPA | [CruiseKube vs VPA](comp-vs-vpa.md) |
| Common questions | [FAQ](overview-faq.md) |
| Install | [Pre-requisites](gs-prerequisites.md) → [Installation](gs-installation.md) |
| Day-2 operations | [Dashboard](config-dashboard.md), [Policies & Modes](operate-policies.md), [Troubleshooting](operate-troubleshooting.md) |
| Internals | [Architecture](arch-introduction.md), [Algorithm](arch-algorithm.md) |

---

## Community

- **GitHub:** [truefoundry/CruiseKube](https://github.com/truefoundry/CruiseKube)  
- **Discord:** [TrueFoundry community](https://discord.gg/Dqek4xJa3N)  
- **Artifact Hub:** [cruisekube Helm chart](https://artifacthub.io/packages/helm/cruisekube/cruisekube)

If CruiseKube fits your cluster, the fastest validation is a normal install, a week of metrics with workloads on **Recommend**, then a small pilot cohort on **Cruise**—then expand with confidence.
