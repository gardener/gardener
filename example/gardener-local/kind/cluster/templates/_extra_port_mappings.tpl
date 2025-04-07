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
{{- if $.Values.gardener.controlPlane.deployed }}
  hostPort: 443
{{- else }}
  # TODO (plkokanov): when using skaffold to deploy, 172.18.255.2 is not used as listenAddress (unlike the local
  #  deployment) because secondary IPs cannot be easily added to inside the `prow` containers. Additionally, there is no
  #  way currently to swap the dns record of the shoot's `kube-apiserver` once it is migrated to this seed.
  hostPort: 9443
{{- end }}
  listenAddress: {{ $listenAddress }}
- containerPort: {{ add 32132 $i }}
  hostPort: 8132
  listenAddress: {{ $listenAddress }}
{{- end }}
{{- end }}
{{- end }}

{{- define "extraPortMappings.gardener.operator.virtualGarden" -}}
{{- if .Values.gardener.garden.deployed -}}
- containerPort: 31443
  hostPort: 443
  listenAddress: 172.18.255.3
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
