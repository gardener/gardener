{{- define "aws-kube2iam.main" -}}
provider "aws" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.SECRET_ACCESS_KEY}"
  region     = "eu-central-1"
}

{{ range $j, $role := .Values.roles }}
resource "aws_iam_role" "role_{{ $j }}" {
  name  = "{{ $role.name }}"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    },
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "AWS": "{{ required "nodesRoleARN is required" $.Values.nodesRoleARN }}"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
}

resource "aws_iam_policy" "policy_{{ $j }}" {
  name        = "{{ $role.name }}"
  description = "{{ $role.description }}"
  policy      = <<EOF
{{ $role.policy }}
EOF
}

resource "aws_iam_role_policy_attachment" "role_attach_{{ $j }}" {
  role       = "${aws_iam_role.role_{{ $j }}.name}"
  policy_arn = "${aws_iam_policy.policy_{{ $j }}.arn}"
}
{{- end -}}
{{- end -}}
