#!/bin/bash

set -e

GENERATE_CONFIGMAP=true
CONFIGMAP_NAME="grafana-datasources"
SERVICE_NAME="autopilot-create-reco"
# SERVICE_NAME="host.docker.internal"

while [[ $# -gt 0 ]]; do
  case $1 in
    --configmap)
      GENERATE_CONFIGMAP=true
      shift
      ;;
    --service-name)
      SERVICE_NAME="$2"
      shift 2
      ;;
    --configmap-name)
      CONFIGMAP_NAME="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  --configmap           Generate Kubernetes ConfigMap instead of files"
      echo "  --service-name NAME   Service name to use in URLs (default: autopilot-create-reco)"
      echo "  --configmap-name NAME ConfigMap name (default: grafana-datasources)"
      echo "  --help               Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# API_URL="https://autopilot-create-reco-autopilot-test-8080.tfy-usea1-ctl.devtest.truefoundry.tech/api/v1/clusters"
# API_URL="http://localhost:8080/api/v1/clusters"
API_URL="https://autopilot-create-reco-autopilot-test-8080.tfy-ctl-euwe1-production.production.truefoundry.com/api/v1/clusters"

if [ "$GENERATE_CONFIGMAP" = true ]; then
    CONFIGMAP_FILE="datasources-configmap.yaml"
else
    INFINITY_FILE="provisioning/datasources/infinity.yaml"
    PROMETHEUS_FILE="provisioning/datasources/prometheus.yaml"
    RECOMMENDATION_FILE="provisioning/datasources/recommendation.yaml"
fi

echo "Fetching clusters from API..."
clusters_json=$(curl -s -u admin:admin "$API_URL")

if [ $? -ne 0 ]; then
    echo "Error: Failed to fetch clusters from API"
    exit 1
fi

clusters=$(echo "$clusters_json" | jq -r '.clusters[] | select(.stats_available == true) | .id')

if [ -z "$clusters" ]; then
    echo "Error: No clusters with stats_available found"
    exit 1
fi

if [ "$GENERATE_CONFIGMAP" = true ]; then
    echo "Generating Kubernetes ConfigMap..."
    cat > "$CONFIGMAP_FILE" << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: $CONFIGMAP_NAME
  labels:
    app: grafana
data:
  infinity.yaml: |
    apiVersion: 1
    datasources:
EOF

    for cluster in $clusters; do
        cat >> "$CONFIGMAP_FILE" << EOF
      - name: Infinity-${cluster}
        type: yesoreyeram-infinity-datasource
        url: http://${SERVICE_NAME}:8080/api/v1/clusters/${cluster}/workload-analysis
        isDefault: false
        editable: true
        basicAuth: true
        basicAuthUser: admin
        jsonData:
          auth_method: none
          allowed_hosts: ["${SERVICE_NAME}", "localhost", "127.0.0.1"]
        secureJsonData:
          basicAuthPassword: admin
EOF
    done

else
    echo "Generating infinity.yaml..."
    cat > "$INFINITY_FILE" << 'EOF'
apiVersion: 1

datasources:
EOF

    for cluster in $clusters; do
        cat >> "$INFINITY_FILE" << EOF
  - name: Infinity-${cluster}
    type: yesoreyeram-infinity-datasource
    url: http://${SERVICE_NAME}:8080/api/v1/clusters/${cluster}/workload-analysis
    isDefault: false
    editable: true
    basicAuth: true
    basicAuthUser: admin
    jsonData:
      auth_method: none
      allowed_hosts: ["localhost", "127.0.0.1", "${SERVICE_NAME}"]
    secureJsonData:
      basicAuthPassword: admin
EOF
    done
fi

if [ "$GENERATE_CONFIGMAP" = true ]; then
    cat >> "$CONFIGMAP_FILE" << 'EOF'
  prometheus.yaml: |
    apiVersion: 1
    datasources:
EOF

    for cluster in $clusters; do
        cat >> "$CONFIGMAP_FILE" << EOF
      - name: Prometheus-${cluster}
        type: prometheus
        access: proxy
        url: http://${SERVICE_NAME}:8080/api/v1/clusters/${cluster}/prometheus-proxy
        isDefault: false
        editable: true
        basicAuth: true
        basicAuthUser: admin
        jsonData:
          timeInterval: 30s
          queryTimeout: 60s
          httpMethod: POST
        secureJsonData:
          basicAuthPassword: admin
EOF
    done

else
    echo "Generating prometheus.yaml..."
    cat > "$PROMETHEUS_FILE" << 'EOF'
apiVersion: 1

datasources:
EOF

    for cluster in $clusters; do
        cat >> "$PROMETHEUS_FILE" << EOF
  - name: Prometheus-${cluster}
    type: prometheus
    access: proxy
    url: http://${SERVICE_NAME}:8080/api/v1/clusters/${cluster}/prometheus-proxy
    isDefault: false
    editable: true
    basicAuth: true
    basicAuthUser: admin
    jsonData:
      timeInterval: 30s
      queryTimeout: 60s
      httpMethod: POST
    secureJsonData:
      basicAuthPassword: admin
EOF
    done
fi

if [ "$GENERATE_CONFIGMAP" = true ]; then
    cat >> "$CONFIGMAP_FILE" << 'EOF'
  recommendation.yaml: |
    apiVersion: 1
    datasources:
EOF

    for cluster in $clusters; do
        cat >> "$CONFIGMAP_FILE" << EOF
      - name: Recommendation-${cluster}
        type: yesoreyeram-infinity-datasource
        url: http://${SERVICE_NAME}:8080/api/v1/clusters/${cluster}/recommendation-analysis
        isDefault: false
        editable: true
        basicAuth: true
        basicAuthUser: admin
        jsonData:
          auth_method: none
          allowed_hosts: ["${SERVICE_NAME}", "localhost", "127.0.0.1"]
        secureJsonData:
          basicAuthPassword: admin
EOF
    done

    echo "Generated Kubernetes ConfigMap: $CONFIGMAP_FILE"
    echo "Apply with: kubectl apply -f $CONFIGMAP_FILE -n <namespace>"

else
    echo "Generating recommendation.yaml..."
    cat > "$RECOMMENDATION_FILE" << 'EOF'
apiVersion: 1

datasources:
EOF

    for cluster in $clusters; do
        cat >> "$RECOMMENDATION_FILE" << EOF
  - name: Recommendation-${cluster}
    type: yesoreyeram-infinity-datasource
    url: http://${SERVICE_NAME}:8080/api/v1/clusters/${cluster}/recommendation-analysis
    isDefault: false
    editable: true
    basicAuth: true
    basicAuthUser: admin
    jsonData:
      auth_method: none
      allowed_hosts: ["localhost", "127.0.0.1", "${SERVICE_NAME}"]
    secureJsonData:
      basicAuthPassword: admin
EOF
    done

    echo "Generated datasource files successfully!"
fi

echo "Clusters with stats available: $(echo $clusters | tr '\n' ' ')"
