{{- define "gcp-clouddns.variables" -}}
variable "SERVICEACCOUNT" {
  description = "ServiceAccount"
  type        = "string"
}
{{- end -}}
