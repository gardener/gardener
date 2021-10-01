---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .LoggerName }}
  namespace: {{ .HelmDeployNamespace }}
  labels:
    app: {{ .AppLabel }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .AppLabel }}
  template:
    metadata:
      labels:
        app: {{ .AppLabel }}
    spec:
      containers:
      - name: logger
        image: k8s.gcr.io/logs-generator:v0.1.1
        args:
          - /bin/sh
          - -c
          - |-
{{ if .DeltaLogsCount }}
            /logs-generator --logtostderr --log-lines-total=${DELTA_LOGS_GENERATOR_LINES_TOTAL} --run-duration=${DELTA_LOGS_GENERATOR_DURATION}
{{- end }}
            /logs-generator --logtostderr --log-lines-total=${LOGS_GENERATOR_LINES_TOTAL} --run-duration=${LOGS_GENERATOR_DURATION}

            # Sleep forever to prevent restarts
            while true; do
              sleep 3600;
            done
        env:
{{ if .DeltaLogsCount }}
        - name: DELTA_LOGS_GENERATOR_LINES_TOTAL
          value: "{{ .DeltaLogsCount }}"
        - name: DELTA_LOGS_GENERATOR_DURATION
{{ if .DeltaLogsDuration }}
          value: "{{ .DeltaLogsDuration }}"
{{ else }}
          value: 0s
{{- end }}
{{- end }}
        - name: LOGS_GENERATOR_LINES_TOTAL
          value: "{{ .LogsCount }}"
        - name: LOGS_GENERATOR_DURATION
{{ if .LogsDuration }}
          value: "{{ .LogsDuration }}"
{{ else }}
          value: 0s
{{- end }}
        resources:
          limits:
            cpu: 8m
            memory: 20Mi
          requests:
            cpu: 4m
            memory: 3Mi
      securityContext:
        fsGroup: 65532
        runAsUser: 65532
        runAsNonRoot: true
{{ if .NodeName }}
      nodeName: {{ .NodeName }}
{{- end }}
{{ if .NodeSelector }}
      nodeSelector: {{ .NodeSelector }}
{{- end }}
