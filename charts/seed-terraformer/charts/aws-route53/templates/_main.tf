{{- define "aws-route53.main" -}}
provider "aws" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.SECRET_ACCESS_KEY}"
  region     = "eu-central-1"
}

//=====================================================================
//= Route53 Record
//=====================================================================

resource "aws_route53_record" "www" {
  zone_id = "{{ required "record.hostedZoneID is required" .Values.record.hostedZoneID }}"
  name    = "{{ required "record.name is required" .Values.record.name }}"
  type    = "{{ if eq (required "record.type is required" .Values.record.type) "ip" }}A{{ else }}CNAME{{ end }}"
  ttl     = "120"
  records = [
{{- include "aws-route53.records" $.Values | trimSuffix "," | indent 4 }}
  ]
}
{{- end -}}
