{{- define "kubeadmConfigPatches" -}}
- |
  kind: ClusterConfiguration
  apiServer:
{{- if .Values.gardener.apiserverRelay.deployed }}  
    certSANs:
      - localhost
      - 127.0.0.1
      - gardener-apiserver.relay.svc.cluster.local
{{- end }}
    extraArgs:
{{- if not .Values.gardener.controlPlane.deployed }}
      authorization-mode: RBAC,Node
{{- else }}
      authorization-mode: RBAC,Node
      authorization-webhook-cache-authorized-ttl: "0"
      authorization-webhook-cache-unauthorized-ttl: "0"
    extraVolumes:
    - name: gardener
      hostPath: /etc/gardener/controlplane/auth-webhook-kubeconfig-{{ if eq .Values.networking.ipFamily "dual" }}ipv4{{ else }}{{ .Values.networking.ipFamily }}{{ end }}.yaml
      mountPath: /etc/gardener/controlplane/auth-webhook-kubeconfig-{{ if eq .Values.networking.ipFamily "dual" }}ipv4{{ else }}{{ .Values.networking.ipFamily }}{{ end }}.yaml
      readOnly: true
      pathType: File
{{- end }}
- |
  apiVersion: kubelet.config.k8s.io/v1beta1
  kind: KubeletConfiguration
  maxPods: 500
  serializeImagePulls: false
  registryPullQPS: 10
  registryBurst: 20
{{- end -}}
