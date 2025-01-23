{{- define "kubeadmConfigPatches" -}}
- |
  # TODO(ialidzhikov): Use the kubeadm.k8s.io/v1beta4 API when kind supports it (ref https://github.com/kubernetes-sigs/kind/pull/3675).
  # Currently kind accepts kubeadm.k8s.io/v1beta3 only.
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
      authorization-config: /etc/gardener-local/kube-apiserver/authz-config.yaml
    extraVolumes:
    - name: authz-config
      mountPath: /etc/gardener-local/kube-apiserver/authz-config.yaml
      readOnly: true
      pathType: File
{{- if and .Values.gardener.controlPlane.deployed .Values.gardener.controlPlane.kindIsGardenCluster }}
      hostPath: /etc/gardener-local/kube-apiserver/authz-config-with-seedauthorizer.yaml
    - name: authz-webhook-kubeconfig
      mountPath: /etc/gardener-local/kube-apiserver/authz-webhook-kubeconfig.yaml
      hostPath: /etc/gardener-local/kube-apiserver/authz-webhook-kubeconfig-{{ if eq .Values.networking.ipFamily "dual" }}ipv6{{ else }}{{ .Values.networking.ipFamily }}{{ end }}.yaml
      readOnly: true
      pathType: File
{{- else }}
      hostPath: /etc/gardener-local/kube-apiserver/authz-config-without-seedauthorizer.yaml
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
