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
      - name: net-nginx
        image: eu.gcr.io/gardener-project/3rd/nginx:1.17.6
        ports:
        - containerPort: 80
      - name: pause
        image: registry.k8s.io/e2e-test-images/agnhost:2.40
        args:
        - pause
      serviceAccountName: {{ .name }}
