---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
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
      initContainers:
      - image: eu.gcr.io/gardener-project/3rd/alpine:3.13.5
        name: data-generator
        command:
        - dd
        - if=/dev/urandom
        - of=/data/data
        - bs=1M
        - count={{ .SizeInMB }}
        volumeMounts:
        - name: source-data
          mountPath: /data
      containers:
      - image: bitnami/kubectl:{{ .KubeVersion }}
        name: hyperkube
        command:
        - sleep
        - "3600"
        env:
        - name: KUBECONFIG
          value: /secret/kubeconfig
        volumeMounts:
        - name: source-data
          mountPath: /data
        - name: kubecfg
          mountPath: /secret
      - image: bitnami/kubectl:{{ .KubeVersion }}
        name: target-container
        command:
        - sleep
        - "3600"
        volumeMounts:
        - name: target-data
          mountPath: /data
      securityContext:
        fsGroup: 65532
        runAsUser: 65532
        runAsNonRoot: true
      volumes:
      - name: target-data
        emptyDir: {}
      - name: source-data
        emptyDir: {}
      - name: kubecfg
        secret:
          secretName: {{ .Name }}
