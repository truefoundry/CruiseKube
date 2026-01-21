---
title: "Frequently Asked Questions"
description: "Common questions about CruiseKube, its capabilities, limitations, and compatibility with different technologies and cluster autoscalers."
keywords:
  - CruiseKube FAQ
  - CruiseKube questions
  - CruiseKube compatibility
  - Kubernetes resource optimization FAQ
---

## Frequently Asked Questions

### Does it work with Java?

The JVM heap is bounded by configured limits (e.g., -Xms/-Xmx and container limits), so CruiseKube can't safely resize Java workloads at runtime; support is therefore limited.

### Do we take memory pressure into account?

Not yet. This is work in progress.

### Does this help in cases without node auto-provisioning?

No. CruiseKube is designed to work with cluster autoscalers that can provision new nodes when needed.

### Why not set CPU limit?

CPU limits can cause throttling issues and prevent workloads from utilizing available CPU capacity during low-traffic periods. [CruiseKube focuses on optimizing CPU requests](../arch-algorithm#why-cpu-limits-arent-needed) while allowing workloads to burst when CPU is available, which aligns with the philosophy that CPU is bursty and shareable.

### Can it work in the same setup as a cluster autoscaler?

Yes, CruiseKube works with other cluster autoscalers, including **Karpenter**.
