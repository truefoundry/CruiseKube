{{/*
Expand the name of the chart.
*/}}
{{- define "cruisekubeController.name" -}}
{{- default "controller" .Values.cruisekubeController.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cruisekubeController.fullname" -}}
{{- if .Values.cruisekubeController.fullnameOverride }}
{{- .Values.cruisekubeController.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default "controller" .Values.cruisekubeController.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cruisekubeController.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cruisekubeController.labels" -}}
helm.sh/chart: {{ include "cruisekubeController.chart" . }}
{{ include "cruisekubeController.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cruisekubeController.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cruisekubeController.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cruisekubeController.serviceAccountName" -}}
{{- if .Values.cruisekubeController.serviceAccount.create }}
{{- default (include "cruisekubeController.fullname" .) .Values.cruisekubeController.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.cruisekubeController.serviceAccount.name }}
{{ end }}
{{- end }}

{{/*
Get image tag
*/}}
{{- define "cruisekubeController.imageTag" -}}
{{- .Values.cruisekubeController.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Name of the Secret that holds controller HTTP basic auth credentials (admin user + password).
When admin.existingSecret is set, that name is used; otherwise a Helm-managed name is used and
a pre-install/pre-upgrade hook Job creates the Secret on first install.
*/}}
{{- define "cruisekubeController.adminSecretName" -}}
{{- if .Values.cruisekubeController.admin.existingSecret }}
{{- .Values.cruisekubeController.admin.existingSecret }}
{{- else }}
{{- printf "%s-admin-credentials" (include "cruisekubeController.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "cruisekubeController.adminCredentialGen.serviceAccountName" -}}
{{- printf "%s-admin-credential-generator" (include "cruisekubeController.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Chart-managed Secret for controller runtime data (usage-telemetry distinct id today; additional keys later).
*/}}
{{- define "cruisekubeController.runtimeDataSecretName" -}}
{{- printf "%s-runtime-data" (include "cruisekubeController.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Data key for the stable usage-telemetry install / distinct_id (must match bootstrap Job and Deployment secretKeyRef).
*/}}
{{- define "cruisekubeController.runtimeDataSecretInstallIdKey" -}}
install-id
{{- end }}

{{/*
Whether HTTP basic auth is enabled. Defaults to true when the key is absent.
Uses hasKey to correctly handle explicit false (Sprig default treats false as empty).
*/}}
{{- define "cruisekubeController.adminEnabled" -}}
{{- $admin := .Values.cruisekubeController.admin | default dict -}}
{{- if hasKey $admin "enabled" -}}
{{- ternary "true" "false" $admin.enabled -}}
{{- else -}}
true
{{- end -}}
{{- end }}

{{/*
Whether the bootstrap hook Job should run (admin and/or runtime-data Secret lifecycle).
*/}}
{{- define "cruisekubeController.bootstrapSecretsJobEnabled" -}}
{{- $ut := .Values.cruisekubeController.usageTelemetry | default dict -}}
{{- $adminEnabled := eq (include "cruisekubeController.adminEnabled" . | trim) "true" -}}
{{- $manageAdmin := and $adminEnabled (not .Values.cruisekubeController.admin.existingSecret) -}}
{{- if and .Values.cruisekubeController.enabled (or $manageAdmin ($ut.enabled | default false)) -}}true{{- end -}}
{{- end }}
{{/*
ServiceMonitor labels - merges common labels with servicemonitor-specific labels
*/}}
{{- define "cruisekubeController.serviceMonitorLabels" -}}
{{- $prometheusLabel := dict "release" "prometheus" }}
{{- $commonLabels := include "cruisekubeController.labels" . | fromYaml }}
{{- $serviceMonitorLabels := mergeOverwrite $commonLabels $prometheusLabel .Values.cruisekubeController.serviceMonitor.additionalLabels }}
{{- toYaml $serviceMonitorLabels }}
{{- end }}

{{/*
Validate that structured metrics provider values are not mixed with equivalent raw env vars.
*/}}
{{- define "cruisekubeController.validateMetricsProviderEnv" -}}
{{- $controllerEnv := .controllerEnv | default dict -}}
{{- $metricsProviderType := .metricsProviderType | default "" -}}
{{- if $metricsProviderType -}}
{{- $structuredMetricsProviderEnvNames := list "CRUISEKUBE_METRICS_PROVIDER" "CRUISEKUBE_METRICS_PROVIDER_URL" "CRUISEKUBE_METRICS_PROVIDER_INSECURE_SKIP_TLS_VERIFY" "CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_TYPE" "CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_URL" "CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_INSECURESKIPTLSVERIFY" "CRUISEKUBE_DEPENDENCIES_LOCAL_METRICSPROVIDER_BEARERTOKEN" "CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_TYPE" "CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_URL" "CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_INSECURESKIPTLSVERIFY" "CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_BEARERTOKEN" -}}
{{- range $envName := $structuredMetricsProviderEnvNames -}}
{{- if hasKey $controllerEnv $envName -}}
{{- fail (printf "cruisekubeController.metricsProvider.type is set, so cruisekubeController.env must not include structured metrics provider env var %s; configure metrics provider only via cruisekubeController.metricsProvider" $envName) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Render structured metrics provider env vars for the in-cluster controller runtime.
*/}}
{{- define "cruisekubeController.metricsProviderEnv" -}}
{{- $metricsProvider := .metricsProvider | default dict -}}
{{- $metricsProviderType := .metricsProviderType | default "" -}}
{{- if $metricsProviderType }}
- name: CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_TYPE
  value: {{ $metricsProviderType | quote }}
- name: CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_URL
  value: {{ $metricsProvider.url | default "" | quote }}
- name: CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_INSECURESKIPTLSVERIFY
  {{- if hasKey $metricsProvider "insecureSkipTLSVerify" }}
  value: {{ $metricsProvider.insecureSkipTLSVerify | quote }}
  {{- else }}
  value: "false"
  {{- end }}
{{- if $metricsProvider.bearerTokenExistingSecret }}
- name: CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_BEARERTOKEN
  valueFrom:
    secretKeyRef:
      name: {{ $metricsProvider.bearerTokenExistingSecret | quote }}
      key: {{ $metricsProvider.bearerTokenExistingSecretKey | required "cruisekubeController.metricsProvider.bearerTokenExistingSecretKey is required when bearerTokenExistingSecret is set" | quote }}
{{- else if $metricsProvider.bearerToken }}
- name: CRUISEKUBE_DEPENDENCIES_INCLUSTER_METRICSPROVIDER_BEARERTOKEN
  value: {{ $metricsProvider.bearerToken | quote }}
{{- end }}
{{- end }}
{{- end }}
