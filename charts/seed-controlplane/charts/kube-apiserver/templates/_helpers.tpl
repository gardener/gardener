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

{{- define "kube-apiserver.admissionPluginConfigFileDir" -}}
/etc/kubernetes/admission
{{- end -}}

{{- define "kube-apiserver.admissionPlugins" }}
{{- range $i, $plugin := .Values.admissionPlugins -}}
{{ $plugin.name }},
{{- end -}}
{{- end -}}

{{- define "kube-apiserver.admissionConfig" }}
{{- range $i, $plugin := .Values.admissionPlugins }}
{{- if $plugin.config }}
- name: {{ $plugin.name }}
  path: {{ include "kube-apiserver.admissionPluginConfigFileDir" . }}/{{ lower $plugin.name }}.yaml
{{- end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.auditversion" -}}
audit.k8s.io/v1
{{- end -}}

{{- define "kube-apiserver.auditConfigAuditPolicy" -}}
{{- if .Values.auditConfig.auditPolicy }}
{{- .Values.auditConfig.auditPolicy -}}
{{- else -}}
apiVersion: {{ include "kube-apiserver.auditversion" . }}
kind: Policy
rules:
- level: None
{{- end -}}
{{- end -}}

{{- define "kube-apiserver.auditConfig.name" -}}
audit-policy-config-{{ include "kube-apiserver.auditConfigAuditPolicy" . | sha256sum | trunc 8 }}
{{- end }}

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

{{- define "kube-apiserver.admissionConfig.data" -}}
admission-configuration.yaml: |
  apiVersion: {{ include "apiserverversion" . }}
  kind: AdmissionConfiguration
  {{- if (include "kube-apiserver.admissionConfig" .) }}
  plugins:
  {{- include "kube-apiserver.admissionConfig" . | indent 2 }}
  {{- else }}
  plugins: []
  {{- end }}

{{- range $i, $plugin := .Values.admissionPlugins }}
{{- if $plugin.config }}
{{ lower $plugin.name }}.yaml: |
{{ toYaml $plugin.config | indent 2 }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "kube-apiserver.admissionConfig.name" -}}
kube-apiserver-admission-config-{{ include "kube-apiserver.admissionConfig.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "kube-apiserver.egressSelector.data" -}}
egress-selector-configuration.yaml: |-
  apiVersion: apiserver.k8s.io/v1alpha1
  kind: EgressSelectorConfiguration
  egressSelections:
  - name: cluster
    connection:
      proxyProtocol: HTTPConnect
      transport:
        tcp:
          url: https://vpn-seed-server:9443
          tlsConfig:
            caBundle: /etc/srv/kubernetes/envoy/ca.crt
            clientCert: /etc/srv/kubernetes/envoy/tls.crt
            clientKey: /etc/srv/kubernetes/envoy/tls.key
  - name: {{ if semverCompare "< 1.20" .Values.kubernetesVersion }}master{{ else }}controlplane{{ end }}
    connection:
      proxyProtocol: Direct
  - name: etcd
    connection:
      proxyProtocol: Direct
{{- end -}}

{{- define "kube-apiserver.egressSelector.name" -}}
kube-apiserver-egress-selector-config-{{ include "kube-apiserver.egressSelector.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "kube-apiserver.serviceAccountSigningKeyConfig.data" -}}
signing-key: {{ .Values.serviceAccountConfig.signingKey | b64enc }}
{{- end -}}

{{- define "kube-apiserver.serviceAccountSigningKeyConfig.name" -}}
kube-apiserver-sa-signing-key-{{ include "kube-apiserver.serviceAccountSigningKeyConfig.data" . | sha256sum | trunc 8 }}
{{- end }}
