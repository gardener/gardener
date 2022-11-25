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
  listenAddress: {{ .Values.gardener.seed.istio.listenAddressDefault }}
- containerPort: 30444
{{- if or (eq .Values.environment "local") .Values.gardener.controlPlane.deployed }}
  hostPort: 443
{{- else }}
  # TODO (plkokanov): when using skaffold to deploy, 127.0.0.2 is not used as listenAddress (unlike the local
  #  deployment) because secondary IPs cannot be easily added to inside the `prow` containers. Additionally, there is no
  #  way currently to swap the dns record of the shoot's `kube-apiserver` once it is migrated to this seed.
  hostPort: 9443
{{- end }}
  listenAddress: {{ .Values.gardener.seed.istio.listenAddressZone0 }}
- containerPort: 30445
{{- if or (eq .Values.environment "local") .Values.gardener.controlPlane.deployed }}
  hostPort: 443
{{- else }}
  # TODO (plkokanov): when using skaffold to deploy, 127.0.0.2 is not used as listenAddress (unlike the local
  #  deployment) because secondary IPs cannot be easily added to inside the `prow` containers. Additionally, there is no
  #  way currently to swap the dns record of the shoot's `kube-apiserver` once it is migrated to this seed.
  hostPort: 9443
{{- end }}
  listenAddress: {{ .Values.gardener.seed.istio.listenAddressZone1 }}
- containerPort: 30446
{{- if or (eq .Values.environment "local") .Values.gardener.controlPlane.deployed }}
  hostPort: 443
{{- else }}
  # TODO (plkokanov): when using skaffold to deploy, 127.0.0.2 is not used as listenAddress (unlike the local
  #  deployment) because secondary IPs cannot be easily added to inside the `prow` containers. Additionally, there is no
  #  way currently to swap the dns record of the shoot's `kube-apiserver` once it is migrated to this seed.
  hostPort: 9443
{{- end }}
  listenAddress: {{ .Values.gardener.seed.istio.listenAddressZone2 }}
{{- end -}}
{{- end -}}

{{- define "extraPortMappings.gardener.operator.virtualGarden" -}}
{{- if .Values.gardener.garden.deployed -}}
- containerPort: 31443
  hostPort: 443
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

{{- define "extraPortMappings.gardener.seed.dns" -}}
{{- if .Values.gardener.controlPlane.deployed -}}
- containerPort: 30053
  hostPort: 53
  protocol: TCP
- containerPort: 30053
  hostPort: 53
  protocol: UDP
{{- end -}}
{{- end -}}
