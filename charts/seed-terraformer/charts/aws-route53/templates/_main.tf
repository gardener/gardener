{{- define "aws-route53.main" -}}
provider "aws" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.SECRET_ACCESS_KEY}"
  region     = "eu-central-1"
}

//=====================================================================
//= Route53 Record
//=====================================================================

{{- range $j, $record := .Values.records }}
resource "aws_route53_record" "www{{ if ne $j 0 }}{{ $j }}{{ end }}" {
  zone_id = "{{ required "hostedZoneID is required" .hostedZoneID }}"
  name    = "{{ required "name is required" .name }}"
  type    = "{{ if eq (required "type is required" .type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl     = "120"
  records = [
{{- include "aws-route53.records" $record | trimSuffix "," | indent 4 }}
  ]
}
{{- end -}}
{{- end -}}

