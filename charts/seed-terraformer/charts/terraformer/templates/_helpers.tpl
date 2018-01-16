{{- define "terraformer.podSpec" -}}
restartPolicy: Never
activeDeadlineSeconds: 1800
containers:
- name: terraform
  image: {{ index .Values.images "terraformer" }}
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - sh /terraform.sh {{.Values.script}}{{if ne .Values.script "validate"}} 2>&1; [[ -f /success ]] && exit 0 || exit 1{{end}}
  resources:
    requests:
      cpu: 100m
  terminationMessagePath: /dev/termination-log
  terminationMessagePolicy: File
  env:
  - name: TF_STATE_CONFIG_MAP_NAME
    value: {{.Values.names.state}}
{{- if .Values.terraformVariablesEnvironment }}
{{ toYaml .Values.terraformVariablesEnvironment | indent 2 }}
{{- end }}
  volumeMounts:
  - mountPath: /tf
    name: tf
  - mountPath: /tfvars
    name: tfvars
  - mountPath: /tf-state-in
    name: tfstate
volumes:
- name: tf
  configMap:
    name: {{.Values.names.configuration}}
- name: tfvars
  secret:
    secretName: {{.Values.names.variables}}
- name: tfstate
  configMap:
    name: {{.Values.names.state}}
{{- end -}}
