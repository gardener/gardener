{{- define "podsecuritypolicies.seccompDefaultProfileName" -}}
runtime/default
{{- end -}}

{{- define "podsecuritypolicies.seccompAllowedProfileNames" -}}
runtime/default,docker/default
{{- end -}}
