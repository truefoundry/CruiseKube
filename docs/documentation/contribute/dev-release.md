---
icon: lucide/tags
title: "Release process"
description: "How maintainers cut CruiseKube releases: version bumps, changelog, GitHub release tags, and Helm chart publishing."
keywords:
  - CruiseKube release
  - versioning
---

# Release process

This page summarizes the **maintainer** workflow. It mirrors the repository’s **[RELEASE.md](https://github.com/truefoundry/CruiseKube/blob/main/RELEASE.md)**—read that file for authoritative, step-by-step detail.

---

## Versioning

CruiseKube follows **[Semantic Versioning](https://semver.org/)**:

- **MAJOR** — incompatible API or behavior changes.  
- **MINOR** — backward-compatible functionality.  
- **PATCH** — backward-compatible fixes.

---

## Preparing a stable release

1. **Bump chart metadata** in `charts/cruisekube/Chart.yaml` — `version` and `appVersion`.  
2. **Pin images** in `charts/cruisekube/values.yaml` (`cruisekubeController`, `cruisekubeWebhook`, `cruisekubeFrontend` tags). Prefer **immutable** digests/tags for production.  
3. **Update `CHANGELOG.md`** — move items from `Unreleased` into `## vX.Y.Z (YYYY-MM-DD)`.  
4. Open a **PR to `main`**, review, and merge.

---

## Publishing

1. Create a **GitHub Release** with tag **`vX.Y.Z`** and human-readable notes (highlights, breaking changes, links to docs).  
2. CI (`.github/workflows/release.yaml`) packages the Helm chart and pushes it to **JFrog Artifactory** (`oci://tfy.jfrog.io/tfy-helm/cruisekube`).

---

## Rollback / hotfix

- **Critical regressions:** cut a **patch** release with fixes; consider marking the bad release in GitHub.  
- **Operators:** `helm rollback` or reinstall the previous chart version with pinned image tags.

---

## Contributors

Every merged change should append a line under **`CHANGELOG.md` → Unreleased**—see [Contributors](dev-contributors.md) and [CONTRIBUTING.md](https://github.com/truefoundry/CruiseKube/blob/main/CONTRIBUTING.md).

---

## Related links

- [CHANGELOG](https://github.com/truefoundry/CruiseKube/blob/main/CHANGELOG.md)  
- [Security disclosure](dev-security.md)  
- [Helm chart reference](../reference/reference-helm-chart.md)
