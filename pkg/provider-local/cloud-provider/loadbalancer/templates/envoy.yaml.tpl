node:
  cluster: cloud-controller-manager-local
  id: cloud-controller-manager-local-id
dynamic_resources:
  cds_config:
    resource_api_version: V3
    path_config_source:
      path: {{ .cdsConfigFilePath }}
  lds_config:
    resource_api_version: V3
    path_config_source:
      path: {{ .ldsConfigFilePath }}
admin:
  access_log:
  - name: envoy.access_loggers.file
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.access_loggers.file.v3.FileAccessLog
      path: /dev/stdout
    filter:
      header_filter:
        header:
          name: ":path"
          exact_match: "/ready"
          invert_match: true
  address:
    pipe:
      path: {{ .envoyAdminSocketPath }}
