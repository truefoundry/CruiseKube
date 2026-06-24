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

Setting `recommendationSettings.hpaResourceAwareOptimization: true` relaxes the
all-or-nothing exclusion to a per-resource one. CruiseKube then right-sizes only
the resource the HPA does **not** scale on, since changing the HPA-managed
resource's request would distort the utilization signal the HPA acts on:

| HPA scales on | CruiseKube right-sizes |
| --- | --- |
| CPU only | Memory (CPU request left untouched) |
| Memory only | CPU (memory request/limit left untouched) |
| CPU **and** memory | Nothing (workload still skipped) |

This requires the HPA to target the resource utilization that is computed
against the container **request** (the standard `Resource` metric source). The
feature is disabled by default; enable it only after validating behavior in
staging.

CruiseKube does not otherwise attempt to coordinate with HPA beyond these
exclusion rules, and users should be cautious when running multiple autonomous
controllers against the same workloads.

---

### Sudden memory spikes can cause short downtime

Memory optimization carries higher risk compared to CPU optimization.

Limitations to be aware of:

* Unexpected memory spikes can exceed assigned limits
* Containers may be OOM killed as a result
* Pod restarts can cause brief service interruptions
* The first occurrence of a new memory usage pattern may result in downtime

This behavior is inherent to memory management in Kubernetes and cannot be fully avoided by dynamic right-sizing systems.

