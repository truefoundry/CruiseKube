{{/*
Expand the name of the chart.
*/}}
{{- define "cruisekube.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cruisekube.fullname" -}}
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
{{- define "cruisekube.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cruisekube.labels" -}}
helm.sh/chart: {{ include "cruisekube.chart" . }}
{{ include "cruisekube.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cruisekube.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cruisekube.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL connection environment variables
*/}}
{{- define "cruisekube.postgresqlEnvVars" -}}
- name: POSTGRES_MODE
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_MODE
{{- if .Values.postgresql.connection.url }}
- name: POSTGRES_URL
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_URL
{{- if .Values.postgresql.connection.password }}
- name: POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-connection
      key: password
{{- end }}
{{- else }}
- name: POSTGRES_HOST
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_HOST
- name: POSTGRES_PORT
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_PORT
- name: POSTGRES_DATABASE
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_DATABASE
- name: POSTGRES_USERNAME
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_USERNAME
- name: POSTGRES_SSL_MODE
  valueFrom:
    configMapKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-config
      key: POSTGRES_SSL_MODE
- name: POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "cruisekube.fullname" . }}-postgresql-connection
      key: password
{{- end }}
{{- end }}

{{/*
Check if PostgreSQL local deployment should be enabled
*/}}
{{- define "cruisekube.postgresqlLocalEnabled" -}}
{{- and .Values.postgresql.local.enabled (not .Values.postgresql.connection.url) (not .Values.postgresql.connection.host) }}
{{- end }}
