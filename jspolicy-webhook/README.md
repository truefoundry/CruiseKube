# JsPolicy Webhook

## Quick Start

### 1. Install Dependencies
```bash
npm install
```

### 2. Compile and Generate Policies
```bash
npm run compile
```

This will generate 2 policy files in the `/policies` directory:
- `adjust-resources.yaml` - Policy definition
- `adjust-resources.bundle.yaml` - Bundled policy with embedded JavaScript

### 3. Apply Policies to Kubernetes
```bash
kubectl apply -f policies/adjust-resources.bundle.yaml
kubectl apply -f policies/adjust-resources.yaml
```