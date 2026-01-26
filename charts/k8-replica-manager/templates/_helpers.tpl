{{/*
Expand the name of the chart.
*/}}
{{- define "k8-replica-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "k8-replica-manager.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "k8-replica-manager.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Chart label (name-version).
*/}}
{{- define "k8-replica-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "k8-replica-manager.labels" -}}
helm.sh/chart: {{ include "k8-replica-manager.chart" . }}
{{ include "k8-replica-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "k8-replica-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k8-replica-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
ServiceAccount name
*/}}
{{- define "k8-replica-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount }}
{{- if .Values.serviceAccount.create -}}
{{- default (include "k8-replica-manager.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- else -}}
{{- include "k8-replica-manager.fullname" . -}}
{{- end -}}
{{- end -}}

{{/*
TLS secret name (either existingSecret or a chart-managed secret).
*/}}
{{- define "k8-replica-manager.tlsSecretName" -}}
{{- if .Values.tls.existingSecret -}}
{{- .Values.tls.existingSecret -}}
{{- else -}}
{{- include "k8-replica-manager.fullname" . -}}-tls
{{- end -}}
{{- end -}}
