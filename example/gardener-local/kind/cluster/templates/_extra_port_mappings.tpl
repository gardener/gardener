{{- define "extraPortMappings.gardener.controlPlane.etcd" -}}
{{- if and .Values.gardener.controlPlane.deployed .Values.gardener.controlPlane.customEtcdStatefulSet -}}
- containerPort: 32379
  hostPort: 32379
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.gardener.selfHostedShoot" -}}
{{- if .Values.gardener.selfHostedShoot.deployed -}}
- containerPort: 30003
  hostPort: 443
  listenAddress: {{ .Values.gardener.selfHostedShoot.listenAddress }}
{{- end -}}
{{- end -}}
