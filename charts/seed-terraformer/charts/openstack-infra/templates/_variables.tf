{{- define "openstack-infra.variables" -}}
variable "USER_NAME" {
  description = "OpenStack user name"
  type        = "string"
}

variable "PASSWORD" {
  description = "OpenStack password"
  type        = "string"
}

variable "CLOUD_CONFIG_DOWNLOADER_KUBECONFIG" {
  description = "Kubeconfig for the Cloud Config Downloader"
  type        = "string"
}
{{- end -}}
