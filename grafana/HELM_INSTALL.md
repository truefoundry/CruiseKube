# Grafana Helm Installation Guide

This guide provides instructions to install an equivalent Grafana setup using Helm that matches your current Docker-based configuration.

## Prerequisites

1. Kubernetes cluster with Helm installed
2. Your autopilot service running in the cluster (accessible as `autopilot-service:8080`)
3. Helm 3.x installed

## Installation Steps

### 1. Add Grafana Helm Repository

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
```

### 2. Create Namespace

```bash
kubectl create namespace monitoring
```

### 3. Apply ConfigMaps

Apply the datasources and dashboard ConfigMaps:

```bash
kubectl apply -f datasources-configmap.yaml -n monitoring
kubectl apply -f dashboard-configmap.yaml -n monitoring
```

### 4. Install Grafana with Helm

```bash
helm install grafana grafana/grafana \
  -f helm-values.yaml \
  -n monitoring \
  --set env.API_URL="http://autopilot-create-reco:8080/clusters"
```

### 5. Get Admin Password

```bash
kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

### 6. Access Grafana

#### Port Forward (for local access):
```bash
kubectl --namespace monitoring port-forward svc/grafana 3000:80
```

Then access Grafana at: http://localhost:3000
- Username: admin
- Password: (from step 5)

#### Or configure Ingress:
Update the `helm-values.yaml` file to enable ingress:

```yaml
ingress:
  enabled: true
  hosts:
    - host: grafana.your-domain.com
      paths:
        - path: /
          pathType: Prefix
```

Then upgrade the release:
```bash
helm upgrade grafana grafana/grafana -f helm-values.yaml -n monitoring
```

## Key Features Replicated

✅ **Direct ConfigMap Mounting**: Datasources and dashboards mounted directly from ConfigMaps  
✅ **Infinity Plugin**: Pre-installed for workload and recommendation analysis  
✅ **Prometheus Datasources**: Pre-configured for each cluster  
✅ **Dashboard Provisioning**: Cluster overview dashboard pre-loaded  
✅ **Simple Configuration**: No sidecars, init containers, or discovery mechanisms needed  

## Configuration Details

### Environment Variables
- `GF_SECURITY_ADMIN_USER`: Admin username (default: admin)
- `GF_SECURITY_ADMIN_PASSWORD`: Admin password (default: admin)

### Datasource Types Provided
1. **Infinity datasources** for each cluster:
   - Workload analysis: `/clusters/{cluster}/workload-analysis`
   - Recommendation analysis: `/clusters/{cluster}/recommendation-analysis`

2. **Prometheus datasources** for each cluster:
   - Metrics proxy: `/clusters/{cluster}/prometheus-proxy`

### Customization

#### Add More Clusters
To add more clusters, edit the `datasources-configmap.yaml` file and add entries for each cluster in all three datasource files (infinity.yaml, prometheus.yaml, recommendation.yaml), then apply:
```bash
kubectl apply -f datasources-configmap.yaml -n monitoring
kubectl rollout restart deployment/grafana -n monitoring
```

#### Update Resources
Edit `helm-values.yaml`:
```yaml
resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi
```

#### Enable Persistence
```yaml
persistence:
  enabled: true
  size: 20Gi
  storageClassName: "fast-ssd"
```

## Troubleshooting

### Check ConfigMap Mounts
```bash
kubectl describe configmap grafana-datasources -n monitoring
```

### Verify ConfigMaps
```bash
kubectl get configmaps -n monitoring
kubectl describe configmap grafana-datasources -n monitoring
```

### Check Datasource Files
```bash
kubectl exec -n monitoring deployment/grafana -- ls -la /etc/grafana/provisioning/datasources/
```

### Check Dashboard Files
```bash
kubectl exec -n monitoring deployment/grafana -- ls -la /etc/grafana/provisioning/dashboards/
```

### Restart Grafana
```bash
kubectl rollout restart deployment/grafana -n monitoring
```

## Uninstall

```bash
helm uninstall grafana -n monitoring
kubectl delete configmap grafana-datasource-generator grafana-dashboards -n monitoring
kubectl delete namespace monitoring
```
