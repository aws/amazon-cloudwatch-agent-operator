{{/*
Expand the name of the chart.
*/}}
{{- define "amazon-cloudwatch-observability.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name for cloudwatch-agent
*/}}
{{- define "cloudwatch-agent.name" -}}
{{- default "cloudwatch-agent" .Values.agent.name }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "amazon-cloudwatch-observability.labels" -}}
{{ include "amazon-cloudwatch-observability.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: EKS
{{- end }}

{{/*
Selector labels
*/}}
{{- define "amazon-cloudwatch-observability.selectorLabels" -}}
app.kubernetes.io/name: {{ include "amazon-cloudwatch-observability.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "amazon-cloudwatch-observability.managerServiceAccountName" -}}
{{- if .Values.manager.serviceAccount.create }}
{{- default (printf "%s-controller-manager" (include "amazon-cloudwatch-observability.name" .)) .Values.manager.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.manager.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cloudwatch-agent.serviceAccountName" -}}
{{- if .Values.agent.enabled }}
{{- default (include "cloudwatch-agent.name" .) .Values.agent.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.agent.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "amazon-cloudwatch-observability.podAnnotations" -}}
{{- if .Values.manager.podAnnotations }}
{{- .Values.manager.podAnnotations | toYaml }}
{{- end }}
{{- end }}

{{- define "amazon-cloudwatch-observability.podLabels" -}}
{{- if .Values.manager.podLabels }}
{{- .Values.manager.podLabels | toYaml }}
{{- end }}
{{- end }}

{{/*
Define the default certificate secret name
*/}}
{{- define "amazon-cloudwatch-observability.certificateSecretName" -}}
{{- default (printf "%s-controller-manager-service-cert" (include "amazon-cloudwatch-observability.name" .)) .Values.admissionWebhooks.secretName }}
{{- end -}}

{{/*
Define the default service name
*/}}
{{- define "amazon-cloudwatch-observability.webhookServiceName" -}}
{{- default (printf "%s-webhook-service" (include "amazon-cloudwatch-observability.name" .)) .Values.manager.service.name }}
{{- end -}}
