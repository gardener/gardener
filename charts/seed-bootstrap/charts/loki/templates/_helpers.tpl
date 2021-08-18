{{- define "loki.config.data" -}}
loki.yaml: |-
  auth_enabled: {{ .Values.authEnabled }}
  ingester:
    chunk_target_size: 1536000
    chunk_idle_period: 3m
    chunk_block_size: 262144
    chunk_retain_period: 3m
    max_transfer_retries: 3
    lifecycler:
      ring:
        kvstore:
          store: inmemory
        replication_factor: 1
  limits_config:
    enforce_metric_name: false
    reject_old_samples: true
    reject_old_samples_max_age: 168h
  schema_config:
    configs:
    - from: 2018-04-15
      store: boltdb
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h
  server:
    http_listen_port: 3100
  storage_config:
    boltdb:
      directory: /data/loki/index
    filesystem:
      directory: /data/loki/chunks
  chunk_store_config:
    max_look_back_period: 360h
  table_manager:
    retention_deletes_enabled: true
    retention_period: 360h
curator.yaml: |-
  LogLevel: info
  DiskPath: /data/loki/chunks
  TriggerInterval: 1h
  InodeConfig:
    MinFreePercentages: 10
    TargetFreePercentages: 15
    PageSizeForDeletionPercentages: 1
  StorageConfig:
    MinFreePercentages: 10
    TargetFreePercentages: 15
    PageSizeForDeletionPercentages: 1
{{- end -}}

{{- define "loki.config.name" -}}
loki-config-{{ include "loki.config.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "telegraf.config.data" -}}
telegraf.conf: |+
  [[outputs.prometheus_client]]
  ## Address to listen on.
  listen = ":9273"
  metric_version = 2
  # Gather packets and bytes throughput from iptables
  [[inputs.iptables]]
  ## iptables require root access on most systems.
  ## Setting 'use_sudo' to true will make use of sudo to run iptables.
  ## Users must configure sudo to allow telegraf user to run iptables with no password.
  ## iptables can be restricted to only list command "iptables -nvL".
  use_sudo = true
  ## defines the table to monitor:
  table = "filter"
  ## defines the chains to monitor.
  ## NOTE: iptables rules without a comment will not be monitored.
  ## Read the plugin documentation for more information.
  chains = [ "INPUT" ]

start.sh: |+
  #/bin/bash

  touch /run/xtables.lock
  iptables -A INPUT -p tcp --dport {{ .Values.kubeRBACProxy.port }}  -j ACCEPT -m comment --comment "promtail"
  /usr/bin/telegraf --config /etc/telegraf/telegraf.conf
{{- end -}}

{{- define "telegraf.config.name" -}}
telegraf-config-{{ include "telegraf.config.data" . | sha256sum | trunc 8 }}
{{- end }}
