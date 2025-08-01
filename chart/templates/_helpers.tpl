{{/*
Expand the name of the chart.
*/}}
{{- define "postgres-upgrade.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "postgres-upgrade.fullname" -}}
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
{{- define "postgres-upgrade.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "postgres-upgrade.labels" -}}
helm.sh/chart: {{ include "postgres-upgrade.chart" . }}
{{ include "postgres-upgrade.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "postgres-upgrade.selectorLabels" -}}
app.kubernetes.io/name: {{ include "postgres-upgrade.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "postgres-upgrade.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "postgres-upgrade.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use for PostgreSQL credentials
*/}}
{{- define "postgres-upgrade.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- printf "%s-secret" (include "postgres-upgrade.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Return PostgreSQL port
*/}}
{{- define "postgres-upgrade.port" -}}
{{- .Values.service.port | default 5432 }}
{{- end }}

{{/*
Return PostgreSQL service name
*/}}
{{- define "postgres-upgrade.serviceName" -}}
{{- include "postgres-upgrade.fullname" . }}
{{- end }}

{{/*
Return the proper PostgreSQL image name
*/}}
{{- define "postgres-upgrade.image" -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.repository .Values.image.tag }}
{{- end }}

{{/*
Compile all warnings into a single message.
*/}}
{{- define "postgres-upgrade.validateValues" -}}
{{- $messages := list -}}
{{- $messages := append $messages (include "postgres-upgrade.validateValues.postgresql.password" .) -}}
{{- $messages := without $messages "" -}}
{{- $message := join "\n" $messages -}}

{{- if $message -}}
{{-   printf "\nVALUES VALIDATION:\n%s" $message | fail -}}
{{- end -}}
{{- end -}}

{{/*
Validate values of PostgreSQL - must provide a password
*/}}
{{- define "postgres-upgrade.validateValues.postgresql.password" -}}
{{- if and (not .Values.postgresql.password) (not .Values.existingSecret) -}}
postgres-upgrade: PostgreSQL password
    A PostgreSQL password is required!
    Please set postgresql.password or provide an existingSecret.
{{- end -}}
{{- end -}}