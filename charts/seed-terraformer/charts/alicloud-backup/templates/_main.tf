{{- define "alicloud-backup.main" -}}
provider "alicloud" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.ACCESS_KEY_SECRET}"
  region     = "{{ required "alicloud.region is required" .Values.alicloud.region }}"
}

//=====================================================================
//= OSS bucket
//=====================================================================

resource "alicloud_oss_bucket" "bucket" {
  bucket        = "{{ required "bucket.name is required" .Values.bucket.name }}"
  acl           = "private"
}

//=====================================================================
//= Output variables
//=====================================================================

output "bucketName" {
  value = "${alicloud_oss_bucket.bucket.id}"
}
{{- end -}}