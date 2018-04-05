{{- define "openstack-infra.dnsServers" }}
{{- if .Values.dnsServers }}
{{- range .Values.dnsServers }}"{{ . }}", {{ end }}
{{- end }}
{{- end -}}
