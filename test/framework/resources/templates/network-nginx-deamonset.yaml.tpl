---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
spec:
  selector:
    matchLabels:
      app: net-nginx
  template:
    metadata:
      labels:
        app: net-nginx
    spec:
      containers:
      - image: eu.gcr.io/gardener-project/3rd/nginx:1.17.5
        name: net-nginx
        ports:
        - containerPort: 80
      - image: eu.gcr.io/gardener-project/3rd/curlimages/curl:7.67.0
        name: net-curl
        command: ["sh", "-c"]
        args: ["sleep 300"]
