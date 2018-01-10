{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler#releases
*/}}
{{- define "imagetag" -}}
{{- if semverCompare ">= 1.8" .Capabilities.KubeVersion.GitVersion -}}
v1.1.0
{{- else -}}
v0.6.2
{{- end -}}
{{- end -}}
