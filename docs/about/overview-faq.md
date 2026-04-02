---
icon: lucide/circle-help
title: "FAQ"
description: "Frequently asked questions about CruiseKube: compatibility, Java, HPA, cluster autoscalers, CPU limits, memory pressure, and security reporting."
keywords:
  - CruiseKube FAQ
  - Kubernetes optimization questions
---

# Frequently asked questions

### What Kubernetes version do I need?

**Kubernetes 1.33 or newer.** CruiseKube relies on **in-place pod resource updates** for continuous optimization without recurring rollouts for request changes. Older clusters may install but will not get the full runtime story.

---

### Does CruiseKube replace HPA or Karpenter?

**No.** It addresses **vertical** sizing (requests/limits policy per pod). **HPA** changes replica count; **Karpenter / Cluster Autoscaler** change node supply. CruiseKube works **alongside** them—see [Use cases](overview-use-cases.md).

---

### What happens if I use CPU- or memory-based HPA?

Workloads using **CPU or memory metrics in HPA** are **skipped** by CruiseKube so the two controllers do not fight. Other HPA modes may still interact indirectly; treat combined behavior as something to validate in staging. More detail in [Tradeoffs](../concepts/arch-tradeoffs.md).

---

### Does it work with Java / JVM workloads?

**Limited.** The JVM heap is bounded by process and container limits; aggressive memory changes at runtime do not always map to a safe live resize. Size Java workloads conservatively and validate behavior. We expect this area to evolve.

---

### Do you account for memory pressure?

**Not yet in the way operators often mean (e.g. full memory pressure integration).** Memory handling today leans on **usage statistics, limits with headroom, and OOM-driven feedback**. Treat this as a roadmap-sensitive area; see [Algorithm — OOM handling](../concepts/arch-algorithm.md#oom-handling).

---

### Why does CruiseKube avoid CPU limits?

CPU **limits** throttle workloads and can hide **contention** behind artificial ceilings. CruiseKube prefers **CPU requests** for fairness and scheduling, uses **PSI** (where available) for contention signal, and documents the tradeoffs in [Why CPU limits aren't needed](../concepts/arch-algorithm.md#why-cpu-limits-arent-needed).

---

### Can CruiseKube help if I do not use node auto-provisioning?

**Much less.** The system assumes the cluster can **grow nodes** when honest demand requires it. Without that, you may still get recommendations, but you cannot rely on the same safety margins when the node pool is fixed and tiny.

---

### Can I run it next to Cluster Autoscaler or Karpenter?

**Yes.** That is the expected deployment model. CruiseKube improves **request honesty**; autoscalers improve **capacity supply**.

---

### Where are container images hosted?

Official charts reference images from **TrueFoundry’s registry** (see [Helm chart reference](../reference/reference-helm-chart.md)). Security policy also mentions **`ghcr.io/truefoundry/cruisekube*`** for reporting scope—confirm your installed chart/image for your environment.

---

### How do I report a security vulnerability?

**Do not open a public GitHub issue.** Email **security@truefoundry.com** with reproduction, impact, and versions. Full policy: [Security](../contribute/dev-security.md) (mirrors the repository `SECURITY.md`).

---

### License?

CruiseKube is distributed under **BUSL-1.1** (see the [license badge](https://github.com/truefoundry/CruiseKube/blob/main/LICENSE) in the README). Review terms before production use.

---

### Where can I get help?

- [Discord](https://discord.gg/Dqek4xJa3N)  
- [GitHub Issues](https://github.com/truefoundry/CruiseKube/issues)  
- Documentation: start at [Introduction](overview.md) and [Troubleshooting](../operate/operate-troubleshooting.md)
