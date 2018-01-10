{{- define "azure-backup.variables" -}}
variable "CLIENT_ID" {
  description = "Azure client id of technical user"
  type        = "string"
}

variable "CLIENT_SECRET" {
  description = "Azure client secret of technical user"
  type        = "string"
}

{{- end -}}
