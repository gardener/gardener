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
      - image: alpine:3.11
        name: load
        command: ["sh", "-c"]
        nodeName: {{ .nodeName }}
        args:
        - while true; do echo "testing"; done;
