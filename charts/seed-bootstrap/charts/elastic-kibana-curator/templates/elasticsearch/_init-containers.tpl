{{- define "init-containers" -}}
# Elasticsearch requires vm.max_map_count to be at least 262144.
- name: sysctl
  image: {{ index .Values.global.images "alpine" }}
  command: ["sh", "-c", "if [ $(sysctl -n vm.max_map_count) -lt 262144 ]; then sysctl -w vm.max_map_count=262144; fi"]
  securityContext:
    privileged: true
- name: chown
  image: {{ index .Values.global.images "elasticsearch-oss" }}
  command: ["sh", "-c", "chown -R elasticsearch:elasticsearch /data"]
  securityContext:
    runAsUser: 0
  volumeMounts:
  - name: elasticsearch-logging
    mountPath: /data
{{- if .Values.searchguard.enabled }}
- name: sgadmin
  image: {{ index .Values.global.images "elasticsearch-searchguard-oss" }}
  command:
   - /bin/sh
   - /usr/share/elasticsearch/config/sgadmin-command.sh
  env:
  - name: PROCESSORS
    valueFrom:
      resourceFieldRef:
        resource: limits.cpu
  resources:
{{- include "util-templates.resource-quantity" .Values.elasticsearch.sgadmin | indent 4 }}
  httpHeaders:
    - name: Authorization
      value: Basic {{ .Values.elasticsearch.readinessProbe.httpAuth }}
  volumeMounts:
  - name: elasticsearch-logging
    mountPath: /data
  - name: config
    mountPath: /usr/share/elasticsearch/config/elasticsearch.yml
    subPath: elasticsearch.yml
  - name: config
    mountPath: /usr/share/elasticsearch/config/log4j2.properties
    subPath: log4j2.properties
  - name: config
    mountPath: /usr/share/elasticsearch/config/jvm.options
    subPath: jvm.options
  - name: searchguard-config
    mountPath: /usr/share/elasticsearch/sgconfig/
  - name: tls-secrets-server
    mountPath: /usr/share/elasticsearch/config/certificates-secrets/server/
  - name: tls-secrets-client
    mountPath: /usr/share/elasticsearch/config/certificates-secrets/client/
  - name: config
    mountPath: /usr/share/elasticsearch/config/sgadmin-command.sh
    subPath: sgadmin-command.sh
{{- end }}
{{- end -}}
