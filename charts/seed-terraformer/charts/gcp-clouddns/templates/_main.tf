{{- define "gcp-clouddns.main" -}}
provider "google" {
  credentials = "${var.SERVICEACCOUNT}"
}

//=====================================================================
//= CloudDNS Record
//=====================================================================

{{- range $j, $record := .Values.records }}
resource "google_dns_record_set" "www{{ if ne $j 0 }}{{ $j }}{{ end }}" {
  managed_zone = "{{ required "hostedZoneID is required" .hostedZoneID }}"
  name         = "{{ required "name is required" .name }}."
  type         = "{{ if eq (required "type is required" .type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl          = 120
  rrdatas      = [
{{- include "gcp-clouddns.records" $record | trimSuffix "," | indent 4 }}
  ]
}
{{- end -}}
{{- end -}}
