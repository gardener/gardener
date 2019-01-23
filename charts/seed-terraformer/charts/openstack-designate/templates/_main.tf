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

{{- range $j, $record := .Values.records }}
resource "openstack_dns_recordset_v2" "www{{ if ne $j 0 }}{{ $j }}{{ end }}" {
  zone_id = "{{ required "hostedZoneID is required" .hostedZoneID }}"
  name = "{{ required "name is required" .name | trimSuffix "."}}."
  type =  "{{ if eq (required "type is required" .type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl = 120
  records = [
{{- include "openstack-designate.records" $record | trimSuffix "," | indent 4 }}
  ]
}
{{- end -}}
{{- end -}}
