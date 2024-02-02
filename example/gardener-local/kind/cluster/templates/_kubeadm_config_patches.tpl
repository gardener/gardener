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
      authorization-mode: RBAC,Node
- |
  apiVersion: kubelet.config.k8s.io/v1beta1
  kind: KubeletConfiguration
  maxPods: 500
  serializeImagePulls: false
  registryPullQPS: 10
  registryBurst: 20
{{- end -}}
