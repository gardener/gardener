{{- define "gcp-infra.variables" -}}
variable "SERVICEACCOUNT" {
  description = "ServiceAccount"
  type        = "string"
}
{{- end -}}
