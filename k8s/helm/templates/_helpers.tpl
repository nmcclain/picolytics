{{/*
Expand the name of the chart.
*/}}
{{- define "picolytics.name" -}}
{{- default .Chart.Name .Values.picolytics.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "picolytics.fullname" -}}
{{- if .Values.picolytics.fullnameOverride }}
{{- .Values.picolytics.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.picolytics.nameOverride }}
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
{{- define "picolytics.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "picolytics.labels" -}}
helm.sh/chart: {{ include "picolytics.chart" . }}
{{ include "picolytics.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "picolytics.selectorLabels" -}}
app.kubernetes.io/name: {{ include "picolytics.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "picolytics.serviceAccountName" -}}
{{- if .Values.picolytics.serviceAccount.create }}
{{- default (include "picolytics.fullname" .) .Values.picolytics.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.picolytics.serviceAccount.name }}
{{- end }}
{{- end }}
