---
icon: lucide/shield-check
title: "Security"
description: "How to report CruiseKube security vulnerabilities, supported versions, and scope for responsible disclosure."
keywords:
  - CruiseKube security
  - vulnerability disclosure
---

# Security

This page adapts the repository **[SECURITY.md](https://github.com/truefoundry/CruiseKube/blob/main/SECURITY.md)** for the documentation site.

---

## Supported versions

| Line | Supported |
|------|-----------|
| **Latest stable** | Yes — security fixes target the current release line. |
| **Older minors** | Generally **not** supported unless explicitly communicated. |

Always run a **recent** chart `appVersion` and image tag in production.

---

## Scope (in-scope)

Responsible disclosure is welcome for:

- The **`cruisekube` application** (controller, webhook, server).  
- **Helm charts** and **Kubernetes manifests** shipped in the official repository.  
- **Container images** published for the project (confirm registry path for your install).

---

## Out of scope

Examples:

- Generic hardening opinions **without** a concrete vulnerability.  
- Issues requiring **cluster-admin** or **root** on nodes as a prerequisite.  
- Third-party dependency bugs (report **upstream** first when appropriate).

---

## How to report

**Do not** file public GitHub issues for undisclosed security problems.

1. Email **[security@truefoundry.com](mailto:security@truefoundry.com)**.  
2. Include: title, affected versions, reproduction steps or PoC, impact (C/I/A), and optional patch ideas.

Maintainers follow **coordinated disclosure**: advisory + release notes when a fix ships, with **credit** unless you prefer anonymity.

---

## Bug bounty

There is **no** cash bounty today; reporters may receive **recognition** and **swag** where appropriate.

---

## Thank you

Security reports help every CruiseKube operator—thank you for taking the time to document issues carefully.
