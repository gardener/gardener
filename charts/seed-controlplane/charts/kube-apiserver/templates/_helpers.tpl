{{- define "kube-apiserver.featureGates" }}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.runtimeConfig" }}
{{- if semverCompare "<= 1.7" .Values.kubernetesVersion }}
- --runtime-config=batch/v2alpha1=true,{{ if .Values.runtimeConfig }}{{ range $config, $enabled := .Values.runtimeConfig }}{{ $config }}={{ $enabled }},{{ end }}{{ end }}
{{- else }}
{{- if .Values.runtimeConfig }}
- --runtime-config={{ range $config, $enabled := .Values.runtimeConfig }}{{ $config }}={{ $enabled }},{{ end }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.oidcConfig" }}
{{- if .Values.oidcConfig }}
{{- if .Values.oidcConfig.issuerURL }}
- --oidc-issuer-url={{ .Values.oidcConfig.issuerURL }}
{{- end }}
{{- if .Values.oidcConfig.clientID }}
- --oidc-client-id={{ .Values.oidcConfig.clientID }}
{{- end }}
{{- if .Values.oidcConfig.caBundle }}
- --oidc-ca-file=/srv/kubernetes/oidc/ca.crt
{{- end }}
{{- if .Values.oidcConfig.usernameClaim }}
- --oidc-username-claim={{ .Values.oidcConfig.usernameClaim }}
{{- end }}
{{- if .Values.oidcConfig.groupsClaim }}
- --oidc-groups-claim={{ .Values.oidcConfig.groupsClaim }}
{{- end }}
{{- if semverCompare ">= 1.8" .Values.kubernetesVersion }}
{{- if .Values.oidcConfig.usernamePrefix }}
- --oidc-username-prefix={{ .Values.oidcConfig.usernamePrefix }}
{{- end }}
{{- if .Values.oidcConfig.groupsPrefix }}
- --oidc-groups-prefix={{ .Values.oidcConfig.groupsPrefix }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}
