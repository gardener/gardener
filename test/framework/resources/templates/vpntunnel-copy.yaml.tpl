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
      - image: eu.gcr.io/gardener-project/3rd/alpine:3.16.1
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
      - image: eu.gcr.io/gardener-project/3rd/alpine:3.16.1
        name: install-kubectl
        command:
        - /bin/sh
        - -c
        - |-
          wget https://storage.googleapis.com/kubernetes-release/release/v{{ .KubeVersion }}/bin/linux/{{ .Architecture }}/kubectl -O /data/kubectl;
          chmod +x /data/kubectl;
        volumeMounts:
        - name: source-data
          mountPath: /data
      containers:
      - image: registry.k8s.io/e2e-test-images/busybox:1.29-4
        name: source-container
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
      - image: registry.k8s.io/e2e-test-images/busybox:1.29-4
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
