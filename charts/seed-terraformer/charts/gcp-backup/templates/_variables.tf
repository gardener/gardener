{{- define "gcp-backup.variables" -}}
variable "SERVICEACCOUNT" {
  description = "ServiceAccount"
  type        = "string"
}
{{- end -}}
