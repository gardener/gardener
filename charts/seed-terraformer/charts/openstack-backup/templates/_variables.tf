{{- define "openstack-backup.variables" -}}
variable "USER_NAME" {
  description = "OpenStack user name"
  type        = "string"
}

variable "PASSWORD" {
  description = "OpenStack password"
  type        = "string"
}
{{- end -}}
