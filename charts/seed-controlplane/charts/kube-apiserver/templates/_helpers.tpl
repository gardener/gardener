{{- define "kube-apiserver.featureGates" }}
{{- if .Values.FeatureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.FeatureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.runtimeConfig" }}
{{- if semverCompare "<= 1.7" .Values.KubernetesVersion }}
- --runtime-config=batch/v2alpha1=true,{{ if .Values.RuntimeConfig }}{{ range $config, $enabled := .Values.RuntimeConfig }}{{ $config }}={{ $enabled }},{{ end }}{{ end }}
{{- else }}
{{- if .Values.RuntimeConfig }}
- --runtime-config={{ range $config, $enabled := .Values.RuntimeConfig }}{{ $config }}={{ $enabled }},{{ end }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.oidcConfig" }}
{{- if .Values.OIDCConfig }}
{{- if .Values.OIDCConfig.issuerURL }}
- --oidc-issuer-url={{ .Values.OIDCConfig.issuerURL }}
{{- end }}
{{- if .Values.OIDCConfig.clientID }}
- --oidc-client-id={{ .Values.OIDCConfig.clientID }}
{{- end }}
{{- if .Values.OIDCConfig.caBundle }}
- --oidc-ca-file=/srv/kubernetes/oidc/ca.crt
{{- end }}
{{- if .Values.OIDCConfig.usernameClaim }}
- --oidc-username-claim={{ .Values.OIDCConfig.usernameClaim }}
{{- end }}
{{- if .Values.OIDCConfig.groupsClaim }}
- --oidc-groups-claim={{ .Values.OIDCConfig.groupsClaim }}
{{- end }}
{{- if semverCompare ">= 1.8" .Values.KubernetesVersion }}
{{- if .Values.OIDCConfig.usernamePrefix }}
- --oidc-username-prefix={{ .Values.OIDCConfig.usernamePrefix }}
{{- end }}
{{- if .Values.OIDCConfig.groupsPrefix }}
- --oidc-groups-prefix={{ .Values.OIDCConfig.groupsPrefix }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}
