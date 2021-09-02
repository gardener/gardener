{{- define "kube-apiserver.featureGates" }}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.runtimeConfig" }}
{{- if .Values.runtimeConfig }}
- --runtime-config={{ range $config, $enabled := .Values.runtimeConfig }}{{ $config }}={{ $enabled }},{{ end }}
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
{{- end -}}

{{- define "kube-apiserver.admissionPlugins" }}
{{- range $i, $plugin := .Values.admissionPlugins -}}
{{ $plugin.name }},
{{- end -}}
{{- end -}}

{{- define "kube-apiserver.serviceAccountConfig" -}}
{{- if .Values.serviceAccountConfig }}
{{- if .Values.serviceAccountConfig.issuer }}
- --service-account-issuer={{ .Values.serviceAccountConfig.issuer }}
{{- end }}
{{- if .Values.serviceAccountConfig.signingKey }}
- --service-account-signing-key-file=/srv/kubernetes/service-account-signing-key/signing-key
- --service-account-key-file=/srv/kubernetes/service-account-signing-key/signing-key
{{- else }}
- --service-account-signing-key-file=/srv/kubernetes/service-account-key/id_rsa
{{- end }}
{{- end -}}
{{- end -}}

{{- define "kube-apiserver.apiAudiences" -}}
{{- if .Values.apiAudiences }}
- --api-audiences={{ .Values.apiAudiences | join "," }}
{{- end -}}
{{- end -}}

{{- define "kube-apiserver.watchCacheSizes" -}}
{{- with .Values.watchCacheSizes }}
{{- if not (kindIs "invalid" .default) }}
- --default-watch-cache-size={{ .default }}
{{- end }}
{{- with .resources }}
- --watch-cache-sizes={{ include "kube-apiserver.resourceWatchCacheSize" . | trimSuffix "," }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.resourceWatchCacheSize" -}}
{{- range . }}
{{- required ".resource is required" .resource }}{{ if .apiGroup }}.{{ .apiGroup }}{{ end }}#{{ .size }},
{{- end -}}
{{- end -}}
