---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: logger
  namespace: {{ .HelmDeployNamespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: logger
  template:
    metadata:
      labels:
        app: logger
    spec:
      containers:
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
          value: 1s
