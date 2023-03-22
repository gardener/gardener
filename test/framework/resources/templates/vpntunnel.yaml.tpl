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
      - image: eu.gcr.io/gardener-project/3rd/curlimages/curl:7.70.0
        name: net-curl
        args:
          - /bin/sh
          - -c
          - |-
            while true; do
              sleep 3600;
            done
      - name: logger
        image: eu.gcr.io/gardener-project/3rd/agnhost:2.40 # Original image registry.k8s.io/e2e-test-images/agnhost:2.40
        args:
          - logs-generator
          - --logtostderr
          - --log-lines-total={{ .LogsCount }}
          - --run-duration={{ .LogsDuration }}
      securityContext:
        fsGroup: 65532
        runAsUser: 65532
        runAsNonRoot: true
