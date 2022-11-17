{{- define "extraMounts.gardener.controlPlane" -}}
{{- if .Values.gardener.controlPlane.deployed }}
- hostPath: example/gardener-local/controlplane
  containerPath: /etc/gardener/controlplane
  readOnly: true
{{- end }}
{{- end -}}

{{- define "extraMounts.backupBucket" -}}
{{- if .Values.backupBucket.deployed -}}
- hostPath: dev/local-backupbuckets
  containerPath: /etc/gardener/local-backupbuckets
{{- end -}}
{{- end -}}

{{- define "extraMounts.registry" -}}
{{- if .Values.registry.deployed }}
- hostPath: dev/local-registry
  containerPath: /etc/gardener/local-registry
{{- end }}
{{- end -}}
