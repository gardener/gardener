{{- define "alicloud-dns.main" -}}
provider "alicloud" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.ACCESS_KEY_SECRET}"
}

//=====================================================================
//= DNS Record
//=====================================================================
resource "alicloud_dns_record" "record" {
  name = "{{ required "record.name is required" .Values.record.name }}"
  host_record = "{{ required "record.hostRecord is required" .Values.record.hostRecord }}"
  type = "{{ if eq (required "record.type is required" .Values.record.type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl = "120"
  value = "{{ required "record.value is required" .Values.record.value }}"
}

{{- end -}}
