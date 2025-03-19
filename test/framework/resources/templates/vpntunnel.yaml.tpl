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
      - name: pause
        image: registry.k8s.io/e2e-test-images/agnhost:2.40
        args:
        - pause
      - name: logger
        image: registry.k8s.io/e2e-test-images/agnhost:2.40
        command: ["/bin/sh", "-c"]
        args:
        - /agnhost logs-generator --logtostderr --log-lines-total={{ .LogsCount }} --run-duration={{ .LogsDuration }} && /agnhost pause
      securityContext:
        fsGroup: 65532
        runAsUser: 65532
        runAsNonRoot: true
