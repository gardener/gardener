apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: kubernetes
    role: pod-anti-affinity
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app: kubernetes
      role: pod-anti-affinity
  template:
    metadata:
      labels:
        app: kubernetes
        role: pod-anti-affinity
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: role
                operator: In
                values:
                - pod-anti-affinity
            topologyKey: "kubernetes.io/hostname"
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
        securityContext:
          runAsUser: 1001
