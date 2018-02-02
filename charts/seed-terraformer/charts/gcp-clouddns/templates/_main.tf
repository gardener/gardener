{{- define "gcp-clouddns.main" -}}
provider "google" {
  credentials = "${var.SERVICEACCOUNT}"
}

//=====================================================================
//= CloudDNS Record
//=====================================================================

resource "google_dns_record_set" "www" {
  managed_zone = "{{ required "record.hostedZoneID is required" .Values.record.hostedZoneID }}"
  name         = "{{ required "record.name is required" .Values.record.name }}."
  type         = "{{ if eq (required "record.type is required" .Values.record.type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl          = 120
  rrdatas      = [
{{- include "gcp-clouddns.records" $.Values | trimSuffix "," | indent 4 }}
  ]
}
{{- end -}}
