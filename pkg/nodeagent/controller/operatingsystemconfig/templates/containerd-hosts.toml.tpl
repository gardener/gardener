# managed by gardener-node-agent
{{- if .server }}
server = {{ .server | quote }}
{{ end }}
{{- range .hostConfigs }}
[host.{{ .hostURL | quote }}]
  capabilities = {{ .capabilities | toJson }}
  {{- if .ca }}
  ca = {{ .ca | toJson }}
  {{- end }}
{{ end }}
