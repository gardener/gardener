apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
data:
  ipv6.conf: |
    server {
        listen [::]:80;
        server_name localhost;
        location / {
            root   /usr/share/nginx/html;
            index  index.html index.htm;
        }
        error_page   500 502 503 504  /50x.html;
        location = /50x.html {
            root   /usr/share/nginx/html;
        }
    }
