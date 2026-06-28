# Security Policy

If you find a security issue in CruiseKube, we want to hear about it. Maintainers triage every report and work with you to get a fix out.

---

## 🛡️ Supported Versions

| Version | Supported | Notes                      |
| ------- | --------- | -------------------------- |
| Latest  | ✅         | Latest stable release line |
| < Latest | ❌         | End‑of‑life                |

> We generally provide security fixes for the latest minor release lines. 

---

## 🔐 Scope

The following components are **in‑scope** for security reporting:

* `cruisekube` main application
* Helm charts and Kubernetes manifests distributed in the official repository
* All container images published under `ghcr.io/truefoundry/cruisekube*`

Out‑of‑scope issues include but are not limited to:

* Third‑party dependencies (report upstream instead)
* Vulnerabilities requiring root or cluster‑admin access
* Best‑practice hardening suggestions without a concrete security impact

---

## 📬 Reporting a Vulnerability

1. **Email** a detailed report to our private list: **[security@truefoundry.com](mailto:security@truefoundry.com)**.
2. Include:

   * A descriptive title (e.g., *"Denial‑of‑Service via oversized HTTP header"*).
   * Affected versions and environment details.
   * Reproduction steps or proof‑of‑concept (PoC) code.
   * Expected vs. actual behavior.
   * Impact assessment (confidentiality, integrity, availability).
   * *Optional* patch or mitigation ideas.
3. *Do NOT* open a public GitHub issue for security problems.

---

## 🔄 Disclosure Policy

* We follow **coordinated disclosure**.
* We publish a GitHub Security Advisory and release notes once a patch is available.
* We credit reporters **unless anonymity is requested**.
* If a vulnerability is found to be already public, we will fast‑track patching and disclosure.

We currently do **not** offer a monetary bug bounty, but we are happy to provide **swag** and public recognition.

---

## 🙏 Thank You

A careful report makes CruiseKube safer for everyone running it. Thanks for taking the time.
