{{- define "kubeadmConfigPatches" -}}
- |
  # TODO(ialidzhikov): Use the kubeadm.k8s.io/v1beta4 API when kind supports it (ref https://github.com/kubernetes-sigs/kind/pull/3675).
  # Currently kind accepts kubeadm.k8s.io/v1beta3 but the resulting kube-system/kubeadm-config ConfigMap uses kubeadm.k8s.io/v1beta4.
  # The ConfigMap is patched in the kind-up.sh script.
  apiVersion: kubeadm.k8s.io/v1beta3
  kind: ClusterConfiguration
  apiServer:
{{- if .Values.gardener.apiserverRelay.deployed }}
    certSANs:
    - localhost
    - 127.0.0.1
    - gardener-apiserver.relay.svc.cluster.local
{{- end }}
    extraArgs:
{{- if or (not .Values.gardener.controlPlane.deployed) (not .Values.gardener.controlPlane.kindIsGardenCluster) }}
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
  serverTLSBootstrap: true
{{- end -}}
