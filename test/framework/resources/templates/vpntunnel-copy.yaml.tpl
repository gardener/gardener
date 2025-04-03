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
      - name: data-generator
        image: registry.k8s.io/e2e-test-images/busybox:1.36.1-1
        command:
        - dd
        - if=/dev/urandom
        - of=/data/data
        - bs=1M
        - count={{ .SizeInMB }}
        volumeMounts:
        - name: source-data
          mountPath: /data
      - name: install-kubectl
        image: registry.k8s.io/e2e-test-images/busybox:1.36.1-1
        command:
        - /bin/sh
        - -c
        - |-
          wget https://dl.k8s.io/release/v{{ .KubeVersion }}/bin/linux/{{ .Architecture }}/kubectl -O /data/kubectl;
          chmod +x /data/kubectl;
        volumeMounts:
        - name: source-data
          mountPath: /data
      containers:
      - name: source-container
        image: registry.k8s.io/e2e-test-images/busybox:1.36.1-1
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
      - name: target-container
        image: registry.k8s.io/e2e-test-images/busybox:1.36.1-1
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
