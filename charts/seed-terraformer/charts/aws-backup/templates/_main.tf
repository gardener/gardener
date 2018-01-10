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

data "aws_iam_policy_document" "s3ObjectOperation" {
  statement {
    actions = [
      "s3:*",
    ]

    principals {
      type = "AWS"

      identifiers = [
        "${aws_iam_user.etcdBackup.arn}",
      ]
    }

    resources = [
      "${aws_s3_bucket.bucket.arn}",
      "${aws_s3_bucket.bucket.arn}/*",
    ]
  }
}

resource "aws_s3_bucket_policy" "bucketPolicy" {
  bucket = "${aws_s3_bucket.bucket.id}"
  policy = "${data.aws_iam_policy_document.s3ObjectOperation.json}"
}

resource "aws_iam_user" "etcdBackup" {
  name = "{{ required "clusterName is required" .Values.clusterName }}-etcd-backup"
}

resource "aws_iam_access_key" "etcdBackupKey" {
  user = "${aws_iam_user.etcdBackup.name}"
}

data "aws_iam_policy_document" "userPolicyDoc" {
  statement {
    actions = [
      "s3:*",
    ]

    resources = [
      "${aws_s3_bucket.bucket.arn}",
      "${aws_s3_bucket.bucket.arn}/*",
    ]
  }
}

resource "aws_iam_user_policy" "etcdBackup" {
  name = "{{ required "clusterName is required" .Values.clusterName }}-etcd-backup"
  user = "${aws_iam_user.etcdBackup.name}"

  policy = "${data.aws_iam_policy_document.userPolicyDoc.json}"
}

//=====================================================================
//= Output variables
//=====================================================================

output "accessKeyID" {
  value = "${aws_iam_access_key.etcdBackupKey.id}"
}

output "secretAccessKey" {
  sensitive = true
  value     = "${aws_iam_access_key.etcdBackupKey.secret}"
}

output "bucketName" {
  value = "${aws_s3_bucket.bucket.id}"
}
{{- end -}}