---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: guestbook
  namespace: {{ .HelmDeployNamespace }}
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: guestbook
    spec:
      containers:
      - image: eu.gcr.io/gardener-project/test/k8s-example-web-app:0.2.0
        name: guestbook
        ports:
        - containerPort: 8080
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
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: guestbook
  namespace: {{ .HelmDeployNamespace }}
  annotations:
    kubernetes.io/ingress.class: nginx
spec:
  rules:
  - host: {{ .ShootDNSHost }}
    http:
      paths:
      - backend:
          serviceName: guestbook
          servicePort: 80
        path: /