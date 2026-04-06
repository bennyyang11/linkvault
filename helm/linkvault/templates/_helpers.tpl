{{/*
Expand the name of the chart.
*/}}
{{- define "linkvault.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "linkvault.fullname" -}}
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
Chart label value (name-version).
*/}}
{{- define "linkvault.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "linkvault.labels" -}}
helm.sh/chart: {{ include "linkvault.chart" . }}
{{ include "linkvault.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "linkvault.selectorLabels" -}}
app.kubernetes.io/name: {{ include "linkvault.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL hostname — embedded or external.
*/}}
{{- define "linkvault.postgresHost" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" (include "linkvault.fullname" .) }}
{{- else }}
{{- .Values.externalPostgresql.host }}
{{- end }}
{{- end }}

{{/*
PostgreSQL port.
*/}}
{{- define "linkvault.postgresPort" -}}
{{- if .Values.postgresql.enabled }}
{{- "5432" }}
{{- else }}
{{- .Values.externalPostgresql.port | default "5432" }}
{{- end }}
{{- end }}

{{/*
Redis address — embedded or external.
*/}}
{{- define "linkvault.redisAddr" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-redis-master:6379" (include "linkvault.fullname" .) }}
{{- else if .Values.externalRedis.host }}
{{- printf "%s:%s" .Values.externalRedis.host (.Values.externalRedis.port | default "6379") }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}
