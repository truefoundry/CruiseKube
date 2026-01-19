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

JVM sets max heap size and that cannot be changed dynamically. So probably not.

### Do we take memory pressure into account?

Not yet. This is work in progress.

### Does this help in cases without node auto-provisioning?

No. CruiseKube is designed to work with cluster autoscalers that can provision new nodes when needed.

### Why not set CPU limit?

CPU limits can cause throttling issues and prevent workloads from utilizing available CPU capacity during low-traffic periods. CruiseKube focuses on optimizing CPU requests while allowing workloads to burst when CPU is available, which aligns with the philosophy that CPU is bursty and shareable.

### Does it work with other cluster autoscalers?

Yes, CruiseKube works with other cluster autoscalers, including **Karpenter**.
