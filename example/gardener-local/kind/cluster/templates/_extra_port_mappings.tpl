{{- define "extraPortMappings.gardener.controlPlane.etcd" -}}
{{- if and .Values.gardener.controlPlane.deployed .Values.gardener.controlPlane.customEtcdStatefulSet -}}
- containerPort: 32379
  hostPort: 32379
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.gardener.seed.istio" -}}
{{- if .Values.gardener.seed.deployed -}}
{{- range $i, $listenAddress := (required ".Values.gardener.seed.istio.listenAddresses is required" .Values.gardener.seed.istio.listenAddresses) }}
- containerPort: {{ add 30443 $i }}
  hostPort: 443
  listenAddress: {{ $listenAddress }}
- containerPort: {{ add 32132 $i }}
  hostPort: 8132
  listenAddress: {{ $listenAddress }}
- containerPort: {{ add 32443 $i }}
  hostPort: 8443
  listenAddress: {{ $listenAddress }}
{{- end }}
{{- end }}
{{- end }}

{{- define "extraPortMappings.gardener.seed.bastion" -}}
{{- if .Values.gardener.seed.deployed -}}
{{- range $i, $listenAddress := (required ".Values.gardener.seed.bastion.listenAddresses is required" .Values.gardener.seed.bastion.listenAddresses) }}
- containerPort: {{ add 30022 $i }}
  hostPort: 22
  listenAddress: {{ $listenAddress }}
{{- end }}
{{- end }}
{{- end }}

{{- define "extraPortMappings.gardener.operator.virtualGarden" -}}
{{- if .Values.gardener.garden.deployed -}}
- containerPort: 31443
  hostPort: 443
  listenAddress: {{ .Values.gardener.garden.virtualGarden.listenAddress }}
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.registry" -}}
{{- if .Values.registry.deployed -}}
- containerPort: 5001
  hostPort: 5001
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.gardener.seed.dns" -}}
{{- if .Values.gardener.controlPlane.deployed -}}
- containerPort: 30053
  hostPort: 5353
  protocol: TCP
  listenAddress: 172.18.255.1
{{- end -}}
{{- end -}}
