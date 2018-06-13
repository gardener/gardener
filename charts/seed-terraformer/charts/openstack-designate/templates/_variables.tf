{{- define "openstack-designate.variables" -}}
variable "OS_AUTH_URL" {
  description = "OpenStack identity url"
  type        = "string"
}

variable "OS_USERNAME" {
  description = "OpenStack username"
  type        = "string"
}

variable "OS_USER_DOMAIN_NAME" {
  description = "OpenStack user domain name"
  type        = "string"
}

variable "OS_PASSWORD" {
  description = "OpenStack password"
  type        = "string"
}

variable "OS_DOMAIN_NAME" {
  description = "OpenStack domain"
  type        = "string"
}

variable "OS_TENANT_NAME" {
  description = "OpenStack project"
  type        = "string"
}
{{- end -}}
