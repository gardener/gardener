{{- define "vpn-shoot.certs.data" -}}
ca.crt: {{ .Values.vpnShootSecretData.ca }}
tls.crt: {{ .Values.vpnShootSecretData.tlsCrt }}
tls.key: {{ .Values.vpnShootSecretData.tlsKey }}
{{- end -}}

{{- define "vpn-shoot.certs.name" -}}
vpn-shoot-{{ include "vpn-shoot.certs.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "vpn-shoot.dh.data" -}}
dh2048.pem: {{ .Values.diffieHellmanKey }}
{{- end -}}

{{- define "vpn-shoot.dh.name" -}}
vpn-shoot-dh-{{ include "vpn-shoot.dh.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "vpn-shoot.tls-auth.data" -}}
vpn.tlsauth: {{ .Values.tlsAuth }}
{{- end -}}

{{- define "vpn-shoot.tls-auth.name" -}}
vpn-shoot-tlsauth-{{ include "vpn-shoot.tls-auth.data" . | sha256sum | trunc 8 }}
{{- end }}
