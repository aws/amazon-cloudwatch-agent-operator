{{/*
Expand the name of the chart.
*/}}
{{- define "cloudwatchagent-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cloudwatchagent-operator.fullname" -}}
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
{{- define "cloudwatchagent-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cloudwatchagent-operator.labels" -}}
{{ include "cloudwatchagent-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: EKS
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cloudwatchagent-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cloudwatchagent-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cloudwatchagent-operator.managerServiceAccountName" -}}
{{- if .Values.manager.serviceAccount.create }}
{{- default (printf "%s-controller-manager" (include "cloudwatchagent-operator.name" .)) .Values.manager.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.manager.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cloudwatchagent-operator.agentServiceAccountName" -}}
{{- if .Values.agent.enabled }}
{{- default (printf "%s-agent" (include "cloudwatchagent-operator.name" .)) .Values.agent.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.agent.serviceAccount.name }}
{{- end }}
{{- end }}


{{- define "cloudwatchagent-operator.podAnnotations" -}}
{{- if .Values.manager.podAnnotations }}
{{- .Values.manager.podAnnotations | toYaml }}
{{- end }}
{{- end }}

{{- define "cloudwatchagent-operator.podLabels" -}}
{{- if .Values.manager.podLabels }}
{{- .Values.manager.podLabels | toYaml }}
{{- end }}
{{- end }}

{{/*
Define the namespace where the resources in the chart will be installed.
*/}}
{{- define "cloudwatchagent-operator.namespace" -}}
amazon-cloudwatch-agent-operator-system
{{- end -}}

{{/*
Define the default certificate name
*/}}
{{- define "cloudwatchagent-operator.certificateName" -}}
{{- default (printf "%s-serving-cert" (include "cloudwatchagent-operator.name" .)) .Values.manager.certificate.name }}
{{- end -}}

{{/*
Define the default certificate secret name
*/}}
{{- define "cloudwatchagent-operator.certificateSecretName" -}}
{{- default (printf "%s-controller-manager-service-cert" (include "cloudwatchagent-operator.name" .)) .Values.manager.certificate.secretName }}
{{- end -}}

{{/*
Define the default service name
*/}}
{{- define "cloudwatchagent-operator.webhookServiceName" -}}
{{- default (printf "%s-webhook-service" (include "cloudwatchagent-operator.name" .)) .Values.manager.service.name }}
{{- end -}}



