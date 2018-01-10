{{- define "azure-infra.variables" -}}
variable "CLIENT_ID" {
  description = "Azure client id of technical user"
  type        = "string"
}

variable "CLIENT_SECRET" {
  description = "Azure client secret of technical user"
  type        = "string"
}

variable "CLOUD_CONFIG_DOWNLOADER_KUBECONFIG" {
  description = "Kubeconfig for the Cloud Config Downloader"
  type        = "string"
}
{{- end -}}
