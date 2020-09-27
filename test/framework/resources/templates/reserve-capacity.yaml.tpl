apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: kubernetes
    role: reserve-capacity
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app: kubernetes
      role: reserve-capacity
  template:
    metadata:
      labels:
        app: kubernetes
        role: reserve-capacity
    spec:
      terminationGracePeriodSeconds: 5
      nodeSelector:
        worker.gardener.cloud/pool: {{ .WorkerPool }}
{{- if .TolerationKey }}
      tolerations:
      - key: {{ .TolerationKey }}
        operator: "Exists"
        effect: "NoSchedule"
{{- end }}
      containers:
      - name: pause-container
        image: gcr.io/google_containers/pause-amd64:3.1
        imagePullPolicy: IfNotPresent
        resources:
          requests:
            cpu: {{ .Requests.CPU }}
            memory: {{ .Requests.Memory }}
          limits:
            cpu: {{ .Requests.CPU }}
            memory: {{ .Requests.Memory }}
        securityContext:
          runAsUser: 1001
