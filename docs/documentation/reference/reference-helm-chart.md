---
icon: lucide/package
title: "Helm chart"
description: "Official CruiseKube Helm chart: OCI install, components, key values, Artifact Hub, and links to the full parameter table."
keywords:
  - CruiseKube Helm
  - OCI helm chart
---

# Helm chart

CruiseKube ships as a **Helm chart** that installs the **controller**, **mutating webhook**, optional **frontend**, and optional **PostgreSQL**.

Dashboard and API access use **HTTP Basic** credentials configured on the controller (`cruisekubeController.admin.*`). For install-time Secret generation, reading credentials with `kubectl`, and password rotation, see [Login & authentication](../operate/authentication.md).

You can view the [`values.yaml`](https://github.com/truefoundry/CruiseKube/blob/main/charts/cruisekube/values.yaml) to know all the possible values for helm chart. 

## Upgrades

```bash
helm upgrade --install cruisekube oci://tfy.jfrog.io/tfy-helm/cruisekube \
  -n cruisekube-system \
  -f your-values.yaml
```

Always read **release notes** and migrate values when bumping **appVersion**—webhook ordering, new env vars, and task defaults change between minors.
