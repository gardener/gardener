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
      - image: registry.k8s.io/e2e-test-images/busybox:1.36.1-1
        name: load
        command: ["sh", "-c"]
        args:
        - while true; do echo "testing"; done;
        securityContext:
          runAsUser: 1001
      {{ if .nodeName }}
      nodeName: {{ .nodeName }}
      {{ end }}
