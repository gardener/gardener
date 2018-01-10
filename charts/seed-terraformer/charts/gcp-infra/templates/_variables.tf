{{- define "gcp-infra.variables" -}}
variable "SERVICEACCOUNT" {
  description = "ServiceAccount"
  type        = "string"
}

variable "CLOUD_CONFIG_DOWNLOADER_KUBECONFIG" {
  description = "Kubeconfig for the Cloud Config Downloader"
  type        = "string"
}
{{- end -}}
