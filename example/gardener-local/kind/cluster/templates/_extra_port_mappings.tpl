{{- define "extraPortMappings.gardener.selfHostedShoot" -}}
{{- if .Values.gardener.selfHostedShoot.deployed -}}
- containerPort: 30003
  hostPort: 443
  listenAddress: {{ .Values.gardener.selfHostedShoot.listenAddress }}
- containerPort: 31443
  hostPort: 443
  listenAddress: {{ .Values.gardener.selfHostedShoot.virtualGarden.listenAddress }}
{{- end -}}
{{- end -}}
