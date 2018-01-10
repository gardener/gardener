{{- define "openstack-infra.terraform" -}}
CLOUD_CONFIG_DOWNLOADER_KUBECONFIG = <<EOF
{{ required ".cloudConfig.kubeconfig is required" .Values.cloudConfig.kubeconfig }}
EOF

# New line is needed! Do not remove this comment.
{{- end -}}
