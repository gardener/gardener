{{- define "config" -}}
# The directory from which notification templates are read.
templates:
- '/etc/alertmanager/template/*.tmpl'

# The root route on which each incoming alert enters.
route:
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
  repeat_interval: 72h

  # Send alerts by default to nowhere
  receiver: dev-null

  routes:
  # email only for critical and blocker
  - match_re:
      visibility: ^(all|owner)$
    receiver: email-kubernetes-ops

inhibit_rules:
# Apply inhibition if the alert name is the same.
- source_match:
    severity: critical
  target_match:
    severity: warning
  equal: ['alertname', 'service', 'cluster']

# Stop all alerts for type=shoot if there are VPN problems.
- source_match:
    service: vpn
  target_match_re:
    type: shoot
  equal: ['type', 'cluster']

# Stop warning and critical alerts if there is a blocker
- source_match:
    severity: blocker
  target_match_re:
    severity: ^(critical|warning)$
  equal: ['cluster']

# If the API server is down inhibit no worker nodes alert. No worker nodes depends on kube-state-metrics which depends on the API server.
- source_match:
    service: kube-apiserver
  target_match_re:
    service: nodes
  equal: ['cluster']

# If API server is down inhibit kube-state-metrics alerts.
- source_match:
    service: kube-apiserver
  target_match_re:
    severity: info
  equal: ['cluster']

# No Worker nodes depends on kube-state-metrics. Inhibit no worker nodes if kube-state-metrics is down.
- source_match:
    service: kube-state-metrics-shoot
  target_match_re:
    service: nodes
  equal: ['cluster']

receivers:
- name: dev-null
- name: email-kubernetes-ops
{{- if .Values.emailConfigs }}
  email_configs:
{{ toYaml .Values.emailConfigs | indent 2 }}
{{- end }}
{{- end -}}
