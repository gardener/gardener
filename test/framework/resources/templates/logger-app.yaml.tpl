---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .LoggerName }}
  namespace: {{ .HelmDeployNamespace }}
  labels:
    origin:                              gardener
    gardener.cloud/role:                 system-component
    resources.gardener.cloud/managed-by: gardener
    app: {{ .AppLabel }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .AppLabel }}
  template:
    metadata:
      labels:
    {{ if .AdditionalLabels }}
      {{- range $key, $value := .AdditionalLabels }}
        {{ $key }}: {{ $value }}
      {{- end }}
    {{ end }}
        app: {{ .AppLabel }}
    spec:
      containers:
      - name: logger
        # A custom agnhost image (europe-docker.pkg.dev/gardener-project/releases/3rd/agnhost) is used instead of the upstream one (registry.k8s.io/e2e-test-images/agnhost)
        # because this Deployment is created in a Seed cluster and the image needs to be signed with particular keys.
        image: europe-docker.pkg.dev/gardener-project/releases/3rd/agnhost:2.40
        command: ["/bin/sh", "-c"]
        args:
        - |-
{{- if .DeltaLogsCount }}
          /agnhost logs-generator --log-lines-total={{ .DeltaLogsCount }} --run-duration={{ .DeltaLogsDuration }}
{{- end }}
          /agnhost logs-generator --log-lines-total={{ .LogsCount }} --run-duration={{ .LogsDuration }}

          sleep infinity
        resources:
          limits:
            cpu: 8m
            memory: 30Mi
          requests:
            cpu: 4m
            memory: 10Mi
      securityContext:
        fsGroup: 65532
        runAsUser: 65532
        runAsNonRoot: true
{{ if .NodeName }}
      nodeName: {{ .NodeName }}
{{- end }}
{{ if .NodeSelector }}
      nodeSelector: {{ .NodeSelector }}
{{- end }}
