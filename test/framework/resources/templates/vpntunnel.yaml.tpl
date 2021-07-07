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
      - image: eu.gcr.io/gardener-project/3rd/curlimages/curl:7.67.0
        name: net-curl
        args:
          - /bin/sh
          - -c
          - |-
            while true; do
              sleep 3600;
            done
      - name: logger
        image: k8s.gcr.io/logs-generator:v0.1.1
        args:
          - /bin/sh
          - -c
          - |-
            /logs-generator --logtostderr --log-lines-total=${LOGS_GENERATOR_LINES_TOTAL} --run-duration=${LOGS_GENERATOR_DURATION}
            # Sleep forever to prevent restarts
            while true; do
              sleep 3600;
            done
        env:
        - name: LOGS_GENERATOR_LINES_TOTAL
          value: "{{ .LogsCount }}"
        - name: LOGS_GENERATOR_DURATION
          value: "{{ .LogsDuration }}"
      securityContext:
        fsGroup: 65532
        runAsUser: 65532
        runAsNonRoot: true
