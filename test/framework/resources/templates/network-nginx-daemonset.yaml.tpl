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
        image: registry.k8s.io/e2e-test-images/nginx:1.15-4
        ports:
        - containerPort: 80
        volumeMounts:
        - name: ipv6-config
          mountPath: /etc/nginx/conf.d/ipv6.conf
          subPath: ipv6.conf
      - name: pause
        image: registry.k8s.io/e2e-test-images/agnhost:2.40
        args:
        - pause
      serviceAccountName: {{ .name }}
      volumes:
      - name: ipv6-config
        configMap:
          name: {{ .name }}
