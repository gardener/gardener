{{- define "aws-infra.variables" -}}
variable "ACCESS_KEY_ID" {
  description = "AWS Access Key ID of technical user"
  type        = "string"
}

variable "SECRET_ACCESS_KEY" {
  description = "AWS Secret Access Key of technical user"
  type        = "string"
}

variable "CLOUD_CONFIG_DOWNLOADER_KUBECONFIG" {
  description = "Kubeconfig for the Cloud Config Downloader"
  type        = "string"
}
{{- end -}}
