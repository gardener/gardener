{{- define "aws-backup.main" -}}
provider "aws" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.SECRET_ACCESS_KEY}"
  region     = "{{ required "aws.region is required" .Values.aws.region }}"
}

//=====================================================================
//= S3 bucket, user, iam_policy
//=====================================================================

resource "aws_s3_bucket" "bucket" {
  bucket        = "{{ required "bucket.name is required" .Values.bucket.name }}"
  acl           = "private"
  force_destroy = true

  tags {
    Name                                        = "{{ required "bucket.name is required" .Values.bucket.name }}"
    "kubernetes.io/cluster/{{ required "clusterName is required" .Values.clusterName }}" = "1"
  }
}

//=====================================================================
//= Output variables
//=====================================================================

output "bucketName" {
  value = "${aws_s3_bucket.bucket.id}"
}
{{- end -}}