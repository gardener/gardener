{{- define "config" -}}
# The directory from which notification templates are read.
templates:
- '/etc/alertmanager/template/*.tmpl'

# The root route on which each incoming alert enters.
route:
  # The labels by which incoming alerts are grouped together.
  group_by: ['type']

  # When a new group of alerts is created by an incoming alert, wait at
  # least 'group_wait' to send the initial notification.
  # This way ensures that you get multiple alerts for the same group that start
  # firing shortly after another are batched together on the first
  # notification.
  group_wait: 1m

  # When the first notification was sent, wait 'group_interval' to send a batch
  # of new alerts that started firing for that group.
  group_interval: 5m

  # If an alert has successfully been sent, wait 'repeat_interval' to
  # resend them.
  repeat_interval: 3h

  # Send alerts by default to nowhere
  receiver: dev-null

  routes:
  # email only for critical and blocker
  - match_re:
      severity: ^(critical|blocker)$
    receiver: email-kubernetes-ops

inhibit_rules:
- source_match:
    severity: critical
  target_match:
    severity: warning
  # Apply inhibition if the alertname is the same.
  equal: ['alertname', 'service']
- source_match:
    severity: critical
  target_match:
    alertname: PrometheusCantScrape
  equal: ['type', 'job']
  # Stop warning and critical alerts, when there is a blocker -
  # no networking, no workers etc.
- source_match:
    severity: blocker
  target_match_re:
    severity: ^(critical|warning)$
  equal: ['type']

receivers:
- name: dev-null
- name: email-kubernetes-ops
{{- if .Values.email_configs }}
  email_configs:
{{ toYaml .Values.email_configs | indent 6 }}
{{- end }}
{{- end -}}