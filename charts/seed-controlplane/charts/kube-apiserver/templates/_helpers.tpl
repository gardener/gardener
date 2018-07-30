{{- define "kube-apiserver.featureGates" }}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- if semverCompare "< 1.11" .Values.kubernetesVersion }}
- --feature-gates=PodPriority=true
{{- end }}
{{- end -}}

{{- define "kube-apiserver.runtimeConfig" }}
{{- if .Values.runtimeConfig }}
- --runtime-config={{ range $config, $enabled := .Values.runtimeConfig }}{{ $config }}={{ $enabled }},{{ end }}
{{- end }}
{{- if semverCompare "< 1.11" .Values.kubernetesVersion }}
- --runtime-config=scheduling.k8s.io/v1alpha1=true
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
{{- if .Values.oidcConfig.signingAlgs }}
- --oidc-signing-algs={{ range $i, $alg := .Values.oidcConfig.signingAlgs }}{{ $alg }}{{ if ne $i (sub (len $.Values.oidcConfig.signingAlgs) 1) }},{{ end }}{{ end }}
{{- end }}
{{- if .Values.oidcConfig.requiredClaims }}
{{- range $key, $val := .Values.oidcConfig.requiredClaims }}
- --oidc-required-claim={{ $key }}={{ $val }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}
