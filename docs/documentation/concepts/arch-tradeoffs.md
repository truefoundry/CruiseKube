---
icon: lucide/scale
title: "CruiseKube Tradeoffs"
description: "Learn about the limitations and constraints of CruiseKube's resource optimization system."
keywords:
  - CruiseKube Tradeoffs
  - Kubernetes resource optimization constraints
  - CruiseKube known issues
  - optimization limitations
---

## Limitations and Tradeoffs

This page outlines known limitations and tradeoffs when running CruiseKube. These are not bugs, but consequences of the design choices CruiseKube makes to optimize pod-level resources.

### Pod eviction can cause disruption

CruiseKube may evict pods when a node cannot safely accommodate the optimized set of workloads.

This has the following implications:

* Pod eviction is inherently disruptive and can lead to temporary unavailability
* Evicted pods will be restarted or rescheduled by Kubernetes
* Latency-sensitive or stateful workloads may observe brief impact if not properly isolated

Operators should assume that enabling CruiseKube introduces eviction as a possible operational outcome, rather than relying exclusively on static over-provisioning to absorb pressure.

---

### Interaction with HPA-enabled workloads can be unpredictable

CruiseKube operates its own control loop, and when combined with other controllers such as Horizontal Pod Autoscaler, the overall system behavior can become harder to reason about.

Key limitations:

* By default, workloads using CPU or memory based HPA are completely skipped by CruiseKube
* For other HPA modes, CruiseKube and HPA may influence the same workloads indirectly
* Competing control loops can lead to oscillations or delayed convergence
* Resource changes and replica scaling may interact in unexpected ways

#### HPA-resource-aware optimization (opt-in)

Setting `recommendationSettings.hpaResourceAwareOptimization: true` replaces the
all-or-nothing exclusion with **coordinated vertical + horizontal scaling**.

The conflict between a VPA-style controller and an HPA on the same resource is
that the request is the *denominator* of the HPA's signal: an HPA on CPU holds
`usage / request ≈ target`, i.e. it drives each pod toward `target × request`
cores. Naively shrinking the request makes utilization rise, so the HPA adds
replicas — the two oscillate.

CruiseKube resolves this by re-deriving the HPA's target so the **absolute
per-pod scale-out point is preserved** when the request is right-sized:

```
setpoint   = targetOld × requestOld         # cores at which the HPA holds steady
requestNew = right-sized request from usage
targetNew  = clamp(round(setpoint / requestNew), 1%, 90%)
```

Because `targetNew × requestNew ≈ targetOld × requestOld`, the HPA keeps making
the same replica decisions in absolute terms while the request now reflects real
usage — delivering recommendations for **both** dimensions.

How it is applied:

| Mode | Vertical (request) | Horizontal (HPA target) |
| --- | --- | --- |
| Recommend | surfaced only | surfaced only |
| Cruise | right-sized in-place by the controller | HPA object patched by the controller |

Notes and limitations:

* Only HPA `Resource` metrics of type **Utilization** (a percentage of the
  request) are coordinated. `AverageValue` targets are independent of the
  request, so the request is right-sized without touching them. Custom/external
  metrics never conflicted and are optimized normally.
* The admission webhook does not mutate HPA objects (it has no transaction
  across both), so it leaves an HPA-managed resource's request unchanged at pod
  creation; the controller delivers the coordinated change shortly after.
* Workloads scaled on **both** CPU and memory are still skipped.
* When a workload was grossly over-requested, `targetNew` is clamped to 90% and
  the per-pod setpoint legitimately drops (the pod runs hotter and the HPA
  scales on real load).
* The feature is disabled by default; enable it only after validating behavior
  in staging, and be cautious running multiple autonomous controllers against
  the same workloads.

---

### Sudden memory spikes can cause short downtime

Memory optimization carries higher risk compared to CPU optimization.

Limitations to be aware of:

* Unexpected memory spikes can exceed assigned limits
* Containers may be OOM killed as a result
* Pod restarts can cause brief service interruptions
* The first occurrence of a new memory usage pattern may result in downtime

This behavior is inherent to memory management in Kubernetes and cannot be fully avoided by dynamic right-sizing systems.

