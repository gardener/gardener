{{- define "podsecuritypolicies.seccompDefaultProfileName" -}}
{{- if semverCompare ">= 1.11" .Values.global.kubernetesVersion -}}
runtime/default
{{- else -}}
docker/default
{{- end -}}
{{- end -}}

{{- define "podsecuritypolicies.seccompAllowedProfileNames" -}}
{{- if semverCompare ">= 1.11" .Values.global.kubernetesVersion -}}
runtime/default,docker/default
{{- else -}}
docker/default
{{- end -}}
{{- end -}}
