{{- define "alicloud-dns.main" -}}
provider "alicloud" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.ACCESS_KEY_SECRET}"
}

//=====================================================================
//= DNS Record
//=====================================================================

{{- range $j, $record := .Values.records }}
resource "alicloud_dns_record" "record{{ if ne $j 0 }}{{ $j }}{{ end }}" {
  name = "{{ required "name is required" .name }}"
  host_record = "{{ required "hostRecord is required" .hostRecord }}"
  type = "{{ if eq (required "type is required" .type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl = "{{ required "ttl is required" .ttl }}"
  value = "{{ required "value is required" .value }}"
}
{{- end -}}
{{- end -}}
