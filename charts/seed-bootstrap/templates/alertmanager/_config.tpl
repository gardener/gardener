{{- define "config" -}}
# The directory from which notification templates are read.
templates:
- '/etc/alertmanager/template/*.tmpl'

# The root route on which each incoming alert enters.
route:
  # The labels by which incoming alerts are grouped together.
  group_by: ['type', 'cluster']

  # When a new group of alerts is created by an incoming alert, wait at
  # least 'group_wait' to send the initial notification.
  # This way ensures that you get multiple alerts for the same group that start
  # firing shortly after another are batched together on the first
  # notification.
  group_wait: 5m

  # When the first notification was sent, wait 'group_interval' to send a batch
  # of new alerts that started firing for that group.
  group_interval: 5m

  # If an alert has successfully been sent, wait 'repeat_interval' to
  # resend them.
  repeat_interval: 12h

  # Send alerts by default to nowhere
  receiver: dev-null

  routes:
  # email only for critical and blocker
  - match_re:
      severity: ^(critical|blocker)$
    receiver: email-kubernetes-ops

inhibit_rules:
# Apply inhibition if the alertname is the same.
- source_match:
    severity: critical
  target_match:
    severity: warning
  equal: ['alertname', 'service', 'cluster']
# Stop all alerts for type=shoot if no there are VPN problems.
- source_match:
    service: vpn
  target_match_re:
    type: shoot
    severity: ^(critical|warning)$
  equal: ['type', 'cluster']
# Stop warning and critical alerts, when there is a blocker -
# no workers, no etcd main etc.
- source_match:
    severity: blocker
  target_match_re:
    severity: ^(critical|warning)$
  equal: ['type', 'cluster']

receivers:
- name: dev-null
- name: email-kubernetes-ops
{{- if $.emailConfigs }}
  email_configs:
{{ toYaml $.emailConfigs | indent 2 }}
{{- end }}
{{- end -}}
