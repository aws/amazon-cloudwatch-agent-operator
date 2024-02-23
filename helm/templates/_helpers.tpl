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
Get the current recommended cloudwatch agent image for a region
*/}}
{{- define "cloudwatch-agent.image" -}}
{{- $imageDomain := "" -}}
{{- $imageDomain = index .Values.agent.image.repositoryDomainMap .Values.region -}}
{{- if not $imageDomain -}}
{{- $imageDomain = .Values.agent.image.repositoryDomainMap.public -}}
{{- end -}}
{{- printf "%s/%s:%s" $imageDomain .Values.agent.image.repository .Values.agent.image.tag -}}
{{- end -}}

{{/*
Get the current recommended cloudwatch agent image for a region
todo: replace this with latest official cw image for Windows
*/}}
{{- define "cloudwatch-agent-windows.image" -}}
{{- default "506463145083.dkr.ecr.us-west-2.amazonaws.com/windows-container-internal:latest" -}}
{{- end -}}

{{/*
Get the current recommended cloudwatch agent operator image for a region
todo: replace this with latest official operator
*/}}
{{- define "cloudwatch-agent-operator.image" -}}
{{- default "506463145083.dkr.ecr.us-west-2.amazonaws.com/cwagent-operator-ciwindows-internal:latest" -}}
{{- end -}}

{{/*
Get the current recommended fluent-bit image for a region
*/}}
{{- define "fluent-bit.image" -}}
{{- $imageDomain := "" -}}
{{- $imageDomain = index .Values.containerLogs.fluentBit.image.repositoryDomainMap .Values.region -}}
{{- if not $imageDomain -}}
{{- $imageDomain = .Values.containerLogs.fluentBit.image.repositoryDomainMap.public -}}
{{- end -}}
{{- printf "%s/%s:%s" $imageDomain .Values.containerLogs.fluentBit.image.repository .Values.containerLogs.fluentBit.image.tag -}}
{{- end -}}

{{/*
Get the current recommended fluent-bit Windows image for a region
*/}}
{{- define "fluent-bit-windows.image" -}}
{{- $imageDomain := "" -}}
{{- $imageDomain = index .Values.containerLogs.fluentBit.image.repositoryDomainMap .Values.region -}}
{{- if not $imageDomain -}}
{{- $imageDomain = .Values.containerLogs.fluentBit.image.repositoryDomainMap.public -}}
{{- end -}}
{{- printf "%s/%s:%s" $imageDomain .Values.containerLogs.fluentBit.image.repository .Values.containerLogs.fluentBit.image.tagWindows -}}
{{- end -}}

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
