{{- define "extraPortMappings.gardener.controlPlane.etcd" -}}
{{- if .Values.gardener.controlPlane.deployed -}}
- containerPort: 32379
  hostPort: 32379
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.gardener.seed.istio" -}}
{{- if .Values.gardener.seed.deployed -}}
- containerPort: 30443
{{- if or (eq .Values.environment "local") .Values.gardener.controlPlane.deployed }}
  hostPort: 443
{{- else }}
  # TODO (plkokanov): when using skaffold to deploy, 127.0.0.2 is not used as listenAddress (unlike the local
  #  deployment) because secondary IPs cannot be easily added to inside the `prow` containers. Additionally, there is no
  #  way currently to swap the dns record of the shoot's `kube-apiserver` once it is migrated to this seed.
  hostPort: 9443
{{- end }}
{{- if eq .Values.environment "local" }}
  listenAddress: {{ .Values.gardener.seed.istio.listenAddress }}
{{- end }}
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.gardener.seed.nginx" -}}
{{- if and .Values.gardener.controlPlane.deployed .Values.gardener.seed.deployed -}}
- containerPort: 30448
  hostPort: 8448
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.registry" -}}
{{- if .Values.registry.deployed -}}
- containerPort: 5001
  hostPort: 5001
{{- end -}}
{{- end -}}
