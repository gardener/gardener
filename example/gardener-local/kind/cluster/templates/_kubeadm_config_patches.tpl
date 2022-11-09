{{- define "kubeadmConfigPatches" -}}
- |
  kind: ClusterConfiguration
  apiServer:
    extraArgs:
{{- if not .Values.gardener.controlPlane.deployed }}
      authorization-mode: RBAC,Node
{{- else }}
      authorization-mode: RBAC,Node,Webhook
      authorization-webhook-config-file: /etc/gardener/controlplane/auth-webhook-kubeconfig-{{ .Values.environment }}.yaml
      authorization-webhook-cache-authorized-ttl: "0"
      authorization-webhook-cache-unauthorized-ttl: "0"
    extraVolumes:
    - name: gardener
      hostPath: /etc/gardener/controlplane/auth-webhook-kubeconfig-{{ .Values.environment }}.yaml
      mountPath: /etc/gardener/controlplane/auth-webhook-kubeconfig-{{ .Values.environment }}.yaml
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
