---
icon: lucide/shield-check
title: "Login & authentication"
description: "How CruiseKube secures the API and UI with Helm-managed credentials, how the admin Secret is created, and how to read or rotate the password."
keywords:
  - CruiseKube login
  - HTTP Basic authentication
  - Helm admin secret
  - password rotation
---

# Login & authentication

CruiseKube protects the **controller HTTP API** (and the dashboard, which proxies to that API) with **HTTP Basic authentication**. Users sign in through the web UI; the browser then sends `Authorization: Basic …` on subsequent API calls.

![CruiseKube sign-in page](/assets/screenshots/feat_login.png)

## Helm values (how credentials are wired)

Credentials always come from a **Kubernetes Secret** in the same namespace as the release. Configure them under `cruisekubeController.admin` in your Helm values file or `--set` flags.

| Value | Purpose |
| --- | --- |
| `cruisekubeController.admin.existingSecret` | Name of a Secret **you** create and manage. When non-empty, the chart **does not** run the bootstrap Job and **does not** generate a password. |
| `cruisekubeController.admin.userKey` | Secret key for the username (default `admin-user`). |
| `cruisekubeController.admin.passwordKey` | Secret key for the password (default `admin-password`). |

The controller Deployment reads that Secret via environment variables (`CRUISEKUBE_SERVER_AUTH_USERNAME` / `CRUISEKUBE_SERVER_AUTH_PASSWORD`).

When `existingSecret` is **empty**, the chart uses a generated Secret name of the form **`{controller-fullname}-admin-credentials`**. For a typical release named `cruisekube`, the controller full name is `cruisekube-controller`, so the Secret is often:

`cruisekube-controller-admin-credentials`

## How the password is generated (first install)

If `cruisekubeController.admin.existingSecret` is **not** set:

1. A **pre-install / pre-upgrade** Helm hook runs a short **bootstrap Job** (resource name `…-bootstrap-secrets`) in the release namespace. It may also create the usage-telemetry install-id Secret when enabled.
2. If the Secret **already exists**, the Job exits without changes (so upgrades keep the same password).
3. If the Secret **does not exist**, the Job creates it with:
   - **Username** fixed to **`cruisekube`** (stored under `userKey`).
   - **Password** a random **32-character** string from `A–Z`, `a–z`, and `0–9` (stored under `passwordKey`).

Implementation lives in the chart template `charts/cruisekube/templates/controller/bootstrap-secrets-job.yaml` (RBAC in `bootstrap-secrets-rbac.yaml`).

## How to read the username and password

### After `helm install` / `helm upgrade`

The chart prints **NOTES** with ready-to-run `kubectl` commands (same idea as below). Read them from the command output.

### Any time (replace namespace and Secret name)

Use the keys from your values (`admin-user` / `admin-password` unless you changed them):

```bash
NAMESPACE=cruisekube-system
SECRET=cruisekube-controller-admin-credentials

kubectl get secret "$SECRET" -n "$NAMESPACE" \
  -o jsonpath='{.data.admin-user}' | base64 -d && echo

kubectl get secret "$SECRET" -n "$NAMESPACE" \
  -o jsonpath='{.data.admin-password}' | base64 -d && echo
```

Sign in to the UI with that username and password. The UI calls `POST /api/v1/auth/login` and then stores the returned Basic token for later requests.

## How to reset the password

Pick one approach.

### Option A — Let the hook recreate credentials (simplest for dev)

1. **Delete** the generated Secret (only do this if you are happy to discard the old password):

   ```bash
   kubectl delete secret cruisekube-controller-admin-credentials -n cruisekube-system
   ```

2. Run **`helm upgrade`** for the same release (same values are fine). The hook Job runs again and **creates a new Secret** with a new random password (username remains `cruisekube`).

3. **Restart** the controller so it reloads env vars from the Secret:

   ```bash
   kubectl rollout restart deployment/cruisekube-controller -n cruisekube-system
   ```

Environment variables populated from `secretKeyRef` are **not** live-updated when the Secret changes; a restart (or new pod) is required.

### Option B — Patch the Secret in place (no Helm hook)

1. Choose a new password string, for example `NEW_PASSWORD`.
2. Base64-encode the **literal string** (not the `username:password` pair) for the key `admin-password` (or your custom `passwordKey`):

   ```bash
   NEW_PASSWORD='...'
   ENC=$(printf '%s' "$NEW_PASSWORD" | base64 | tr -d '\n')
   kubectl patch secret cruisekube-controller-admin-credentials -n cruisekube-system \
     --type merge -p "{\"data\":{\"admin-password\":\"$ENC\"}}"
   ```

3. **Restart** the controller Deployment as above.

### Option C — Bring your own Secret (`existingSecret`)

1. Create a Secret with your chosen keys (`userKey` / `passwordKey`).
2. Set `cruisekubeController.admin.existingSecret` to that Secret’s name.
3. Upgrade the release and restart the controller if needed.

## Related topics

- [Basic usage](../../install/basic-usage.md) — port-forward to the frontend service.
- [Configuration dashboard](config-dashboard.md) — using the UI after sign-in.
- [Helm chart reference](../reference/reference-helm-chart.md) — full values list.
