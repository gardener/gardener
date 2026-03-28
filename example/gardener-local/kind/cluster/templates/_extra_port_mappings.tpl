{{- define "extraPortMappings.gardener.controlPlane.etcd" -}}
{{- if and .Values.gardener.controlPlane.deployed .Values.gardener.controlPlane.customEtcdStatefulSet -}}
- containerPort: 32379
  hostPort: 32379
{{- end -}}
{{- end -}}
