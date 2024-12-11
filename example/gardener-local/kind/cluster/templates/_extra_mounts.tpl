{{- define "extraMounts.kind.kubeAPIServer" -}}
- hostPath: {{.Values.gardener.repositoryRoot}}/example/gardener-local/kube-apiserver
  containerPath: /etc/gardener-local/kube-apiserver
  readOnly: true
{{- end -}}

{{- define "extraMounts.gardener.controlPlane" -}}
{{- if and .Values.gardener.controlPlane.deployed .Values.gardener.controlPlane.kindIsGardenCluster }}
- hostPath: {{.Values.gardener.repositoryRoot}}/example/gardener-local/controlplane
  containerPath: /etc/gardener/controlplane
  readOnly: true
{{- end }}
{{- end -}}

{{- define "extraMounts.backupBucket" -}}
{{- if .Values.backupBucket.deployed -}}
- hostPath: {{.Values.gardener.repositoryRoot}}/dev/local-backupbuckets
  containerPath: /etc/gardener/local-backupbuckets
{{- end -}}
{{- end -}}

{{- define "extraMounts.registry" -}}
{{- if .Values.registry.deployed }}
- hostPath: {{.Values.gardener.repositoryRoot}}/dev/local-registry
  containerPath: /etc/gardener/local-registry
{{- end }}
{{- end -}}
