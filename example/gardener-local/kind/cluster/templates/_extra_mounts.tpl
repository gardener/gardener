{{- define "extraMounts.backupBucket" -}}
- hostPath: {{.Values.repositoryRoot}}/dev/local-backupbuckets
  containerPath: /etc/gardener/local-backupbuckets
{{- end -}}

{{- define "extraMounts.resolveConfig" -}}
- hostPath: {{.Values.repositoryRoot}}/example/gardener-local/kind/resolv.conf
  containerPath: /etc/resolv.conf
{{- end -}}

{{- define "extraMounts.dockerSocket" -}}
- hostPath: {{ .Values.dockerSocket | default "/var/run/docker.sock" }}
  containerPath: /var/run/docker.sock
{{- end -}}
