{{/*
Expand the name of the chart.
*/}}
{{- define "autopilot.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "autopilot.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
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
{{- define "autopilot.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "autopilot.labels" -}}
helm.sh/chart: {{ include "autopilot.chart" . }}
{{ include "autopilot.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "autopilot.selectorLabels" -}}
app.kubernetes.io/name: {{ include "autopilot.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "autopilot.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "autopilot.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "autopilot.certGenServiceAccountName" -}}
{{- if .Values.certGen.serviceAccount.create }}
{{- default (printf "%s-certgen" (include "autopilot.fullname" .)) .Values.certGen.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.certGen.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the TLS secret
*/}}
{{- define "autopilot.tlsSecretName" -}}
{{- printf "%s-tls" (include "autopilot.fullname" .) }}
{{- end }}


{{/*
Create the webhook service name
*/}}
{{- define "autopilot.webhookServiceName" -}}
{{- printf "%s-webhook" (include "autopilot.fullname" .) }}
{{- end }}

{{/*
Generate namespace selector for webhook
*/}}
{{- define "autopilot.mutatingWebhookConfigurationNamespaceSelector" -}}
matchExpressions:
- key: name
  operator: NotIn
  values:
  {{- range .Values.mutatingWebhookConfiguration.namespaceSelector.excludeNamespaces }}
  - {{ . | quote }}
  {{- end }}
{{- end }}


{{/*
Get image tag
*/}}
{{- define "autopilot.imageTag" -}}
{{- .Values.image.tag | default .Chart.AppVersion }}
{{- end }}
{{/*
ServiceMonitor labels - merges common labels with servicemonitor-specific labels
*/}}
{{- define "autopilot.serviceMonitorLabels" -}}
{{- $prometheusLabel := dict "release" "prometheus" }}
{{- $commonLabels := include "autopilot.labels" . | fromYaml }}
{{- $serviceMonitorLabels := mergeOverwrite $commonLabels $prometheusLabel .Values.serviceMonitor.additionalLabels }}
{{- toYaml $serviceMonitorLabels }}
{{- end }}
