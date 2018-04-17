<%
  # Arguments
  # namespace:         The namespace into which the contents of this file should be deployed
  # shoot-dns-suffix:  The DNS Suffix of the Shoot

  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")))

  def value(path, default):
    keys=str.split(path, ".")
    root=values
    for key in keys:
      if isinstance(root, dict):
        if key in root:
          root=root[key]
        else:
          return default
      else:
        return default
    return root

  guestbook_ingress_host="guestbook.ingress.{}".format(shootDnsHost)
%>---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: guestbook
  namespace: ${value("metadata.namespace", namespace)}
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
  namespace: ${value("metadata.namespace", namespace)}
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
  namespace: ${value("metadata.namespace", namespace)}
  annotations:
    kubernetes.io/ingress.class: nginx
spec:
  rules:
  - host: ${value("spec.rules[0].host", guestbook_ingress_host)}
    http:
      paths:
      - backend:
          serviceName: guestbook
          servicePort: 80
        path: /