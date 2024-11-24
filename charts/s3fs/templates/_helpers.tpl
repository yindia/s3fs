{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "s3fs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels (immutable)
*/}}
{{- define "s3fs.immutableSelectorLabels" -}}
app.kubernetes.io/name: {{ include "s3fs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}



{{/*
Server selector labels (immutable)
*/}}
{{- define "s3fs.server.immutableSelectorLabels" -}}
{{ include "s3fs.immutableSelectorLabels" . }}
app.kubernetes.io/component: server
{{- end }}


{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "s3fs.fullname" -}}
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
{{- define "s3fs.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "s3fs.labels" -}}
helm.sh/chart: {{ include "s3fs.chart" . }}
{{ include "s3fs.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "s3fs.selectorLabels" -}}
app.kubernetes.io/name: {{ include "s3fs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Server labels
*/}}
{{- define "s3fs.server.labels" -}}
{{ include "s3fs.labels" . }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Server selector labels
*/}}
{{- define "s3fs.server.selectorLabels" -}}
{{ include "s3fs.selectorLabels" . }}
app.kubernetes.io/component: server
{{- end }}



{{/*
Create the name of the service account to use
*/}}
{{- define "s3fs.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "s3fs.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}