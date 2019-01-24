{{- define "alicloud-infra.variables" -}}
variable "ACCESS_KEY_ID" {
  description = "Alicloud access key id"
  type        = "string"
}

variable "ACCESS_KEY_SECRET" {
  description = "Alicloud access key secret"
  type        = "string"
}
{{- end -}}
