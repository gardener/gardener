{{- define "alicloud-backup.variables" -}}
variable "ACCESS_KEY_ID" {
  description = "Alicloud Access Key ID of technical user"
  type        = "string"
}

variable "ACCESS_KEY_SECRET" {
  description = "Alicloud Secret Access Key of technical user"
  type        = "string"
}
{{- end -}}