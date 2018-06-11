{{- define "openstack-designate.main" -}}
provider "openstack" {
  auth_url           = "${var.OS_AUTH_URL}"
  domain_name        = "${var.OS_DOMAIN_NAME}"
  tenant_name        = "${var.OS_TENANT_NAME}"
  user_name          = "${var.OS_USERNAME}"
  user_domain_name   = "${var.OS_USER_DOMAIN_NAME}"
  password           = "${var.OS_PASSWORD}"
  insecure           = true
}

//=====================================================================
//= Route53 Record
//=====================================================================

resource "openstack_dns_recordset_v2" "www" {
  zone_id = "{{ required "record.hostedZoneID is required" .Values.record.hostedZoneID }}"
  name = "{{ required "record.name is required" .Values.record.name | trimSuffix "."}}."
  type =  "{{ if eq (required "record.type is required" .Values.record.type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl = 120
  records = [
{{- include "openstack-designate.records" $.Values | trimSuffix "," | indent 4 }}
  ]
}
{{- end -}}
