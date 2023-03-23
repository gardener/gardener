{{- define "containerdConfigPatches" -}}
- |-
  {{- if eq .Values.environment "skaffold" }}
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
    endpoint = ["http://{{ .Values.registry.hostname }}:5001"]
  {{- end }}
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."gcr.io"]
    endpoint = ["http://{{ .Values.registry.hostname }}:5003"]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."eu.gcr.io"]
    endpoint = ["http://{{ .Values.registry.hostname }}:5004"]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."ghcr.io"]
    endpoint = ["http://{{ .Values.registry.hostname }}:5005"]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.k8s.io"]
    endpoint = ["http://{{ .Values.registry.hostname }}:5006"]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."quay.io"]
    endpoint = ["http://{{ .Values.registry.hostname }}:5007"]
{{- end -}}
