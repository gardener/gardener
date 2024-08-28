---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: guestbook
  namespace: {{ .HelmDeployNamespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: guestbook
  template:
    metadata:
      labels:
        app: guestbook
    spec:
      containers:
      - image: europe-docker.pkg.dev/gardener-project/releases/test/k8s-example-web-app:0.4.0
        name: guestbook
        ports:
        - containerPort: 8080
        securityContext:
          runAsUser: 1001
        env:
        - name: REDIS_SERVICE_NAME
          value: redis-master
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis
              key: redis-password
---
kind: Service
apiVersion: v1
metadata:
  name: guestbook
  namespace: {{ .HelmDeployNamespace }}
spec:
  selector:
    app: guestbook
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: guestbook
  namespace: {{ .HelmDeployNamespace }}
spec:
  ingressClassName: nginx
  rules:
  - host: {{ .ShootDNSHost }}
    http:
      paths:
      - backend:
          service:
            name: guestbook
            port:
              number: 80
        path: /
        pathType: Prefix