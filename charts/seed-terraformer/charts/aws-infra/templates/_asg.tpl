{{- define "aws-infra.asg-policies" -}}
{{- range $j, $worker := .workers }}
{{- range $i, $zone := $worker.zones }}
"${aws_autoscaling_group.nodes_pool{{ $j }}_z{{ $i }}.arn}",
{{- end -}}
{{- end -}}
{{- end -}}