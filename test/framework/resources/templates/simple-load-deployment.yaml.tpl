---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
spec:
  selector:
    matchLabels:
      app: load
  template:
    metadata:
      labels:
        app: load
    spec:
      containers:
      - image: eu.gcr.io/gardener-project/3rd/alpine:3.16.1
        name: load
        command: ["sh", "-c"]
        args:
        - while true; do echo "testing"; done;
        securityContext:
          runAsUser: 1001
      {{ if .nodeName }}
      nodeName: {{ .nodeName }}
      {{ end }}
