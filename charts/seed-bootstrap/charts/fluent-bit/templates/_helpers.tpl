{{- define "fluent-bit.config.data" -}}
fluent-bit.conf: |-
  # Service section
{{- include "fluent-bit.conf" . }}

input.conf: |-
  # Input section
{{- include "input.conf" . }}

filter-kubernetes.conf: |-
  # Lua filter to add the tag into the record
  [FILTER]
      Name                lua
      Match               kubernetes.*
      script              add_tag_to_record.lua
      call                add_tag_to_record

  # Systemd Filters
  [FILTER]
      Name record_modifier
      Match journald.docker
      Record hostname ${NODE_NAME}
      Record unit docker

  [FILTER]
      Name record_modifier
      Match journald.containerd
      Record hostname ${NODE_NAME}
      Record unit containerd

  [FILTER]
      Name record_modifier
      Match journald.kubelet
      Record hostname ${NODE_NAME}
      Record unit kubelet

  [FILTER]
      Name record_modifier
      Match journald.cloud-config-downloader*
      Record hostname ${NODE_NAME}
      Record unit cloud-config-downloader

  [FILTER]
      Name record_modifier
      Match journald.docker-monitor
      Record hostname ${NODE_NAME}
      Record unit docker-monitor

  [FILTER]
      Name record_modifier
      Match journald.containerd-monitor
      Record hostname ${NODE_NAME}
      Record unit containerd-monitor

  [FILTER]
      Name record_modifier
      Match journald.kubelet-monitor
      Record hostname ${NODE_NAME}
      Record unit kubelet-monitor

  # Shoot controlplane filters
  [FILTER]
      Name                parser
      Match               kubernetes.*addons-kubernetes-dashboard*kubernetes-dashboard*
      Key_Name            log
      Parser              kubernetesdashboardParser
      Reserve_Data        True

  # System components filters
  [FILTER]
      Name                parser
      Match               kubernetes.*addons-nginx-ingress-controller*nginx-ingress-controller*
      Key_Name            log
      Parser              kubeapiserverParser
      Reserve_Data        True

  [FILTER]
      Name                parser
      Match               kubernetes.*node-exporter*node-exporter*
      Key_Name            log
      Parser              nodeexporterParser
      Reserve_Data        True

  # Garden filters
  [FILTER]
      Name                parser
      Match               kubernetes.*alertmanager*alertmanager*
      Key_Name            log
      Parser              alertmanagerParser
      Reserve_Data        True

  [FILTER]
      Name                parser
      Match               kubernetes.*prometheus*prometheus*
      Key_Name            log
      Parser              prometheusParser
      Reserve_Data        True

  [FILTER]
      Name                parser
      Match               kubernetes.*prometheus*blackbox-exporter*
      Key_Name            log
      Parser              prometheusParser
      Reserve_Data        True

  [FILTER]
      Name                parser
      Match               kubernetes.*grafana*grafana*
      Key_Name            log
      Parser              grafanaParser
      Reserve_Data        True

  [FILTER]
      Name                parser
      Match               kubernetes.*loki*loki*
      Key_Name            log
      Parser              lokiParser
      Reserve_Data        True

  [FILTER]
      Name                parser
      Match               kubernetes.*loki*curator*
      Key_Name            log
      Parser              lokiCuratorParser
      Reserve_Data        True

  # Extension filters
  [FILTER]
      Name                parser
      Match               kubernetes.*gardener-extension*
      Key_Name            log
      Parser              extensionsParser
      Reserve_Data        True

  [FILTER]
      Name                modify
      Match               kubernetes.*gardener-extension*
      Rename              level  severity
      Rename              msg    log
      Rename              logger source

  # Extensions
{{ if .Values.additionalFilters }}
{{- toString .Values.additionalFilters | indent 2 }}
{{- end }}

  # Scripts
  [FILTER]
      Name                lua
      Match               kubernetes.*
      script              modify_severity.lua
      call                cb_modify

output.conf: |-
  # Output section
{{- include "output.conf" . }}

parsers.conf: |-
  # Custom parsers
  [PARSER]
      Name        docker
      Format      json
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L%z
      Time_Keep   On
      # Command      |  Decoder | Field | Optional Action
      # =============|==================|=================
      Decode_Field_As   json       log

  [PARSER]
      Name        containerd
      Format      regex
      Regex       ^(?<time>[^ ]+) (stdout|stderr) ([^ ]*) (?<log>.*)$
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L%z
      Time_Keep   On
      # Command      |  Decoder | Field | Optional Action
      # =============|==================|=================
      Decode_Field_As   json       log

  [PARSER]
      Name        kubeapiserverParser
      Format      regex
      Regex       ^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)$
      Time_Key    time
      Time_Format %m%d %H:%M:%S.%L

  [PARSER]
      Name        alertmanagerParser
      Format      regex
      Regex       ^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt].*[zZ])\s+caller=(?<source>[^\s]*+)\s+(?<log>.*)
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L

  [PARSER]
      Name        kubernetesdashboardParser
      Format      regex
      Regex       ^(?<time>\d{4}\/\d{2}\/\d{2}\s+[^\s]*)\s+(?<log>.*)
      Time_Key    time
      Time_Format %Y/%m/%d %H:%M:%S

  [PARSER]
      Name        nodeexporterParser
      Format      regex
      Regex       ^time="(?<time>\d{4}-\d{2}-\d{2}T[^"]*)"\s+level=(?<severity>\w+)\smsg="(?<log>.*)"\s+source="(?<source>.*)"
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L

  [PARSER]
      Name        grafanaParser
      Format      regex
      Regex       ^t=(?<time>\d{4}-\d{2}-\d{2}T[^ ]*)\s+lvl=(?<severity>\w+)\smsg="(?<log>.*)"\s+logger=(?<source>.*)
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S%z

  [PARSER]
      Name        lokiParser
      Format      regex
      Regex       ^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt]{1}\d{2}:\d{2}:\d{2}\.\d+\S+?)\S*?\s+caller=(?<source>.*?)\s+(?<log>.*)$
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L%z

  [PARSER]
      Name        prometheusParser
      Format      regex
      Regex       ^ts=(?<time>\d{4}-\d{2}-\d{2}[Tt]{1}\d{2}:\d{2}:\d{2}\.\d+\S+)\s+caller=(?<source>.+?)\s+level=(?<severity>\w+)\s+(?<log>.*)$
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L%z

  [PARSER]
      Name        lokiCuratorParser
      Format      regex
      Regex       ^level=(?<severity>\w+)\s+caller=(?<source>.*?)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt]{1}\d{2}:\d{2}:\d{2}\.\d+\S+?)\S*?\s+(?<log>.*)$
      Time_Key    time
      Time_Format %Y-%m-%dT%H:%M:%S.%L%z

  [PARSER]
      Name        extensionsParser
      Format      json
      Time_Key    ts
      Time_Format %Y-%m-%dT%H:%M:%S

{{ if .Values.additionalParsers }}
{{- toString .Values.additionalParsers | indent 2 }}
{{- end }}

plugin.conf: |-
  [PLUGINS]
      Path /fluent-bit/plugins/out_loki.so

modify_severity.lua: |-
  function cb_modify(tag, timestamp, record)
    local unified_severity = cb_modify_unify_severity(record)

    if not unified_severity then
      return 0, 0, 0
    end

    return 1, timestamp, record
  end

  function cb_modify_unify_severity(record)
    local modified = false
    local severity = record["severity"]
    if severity == nil or severity == "" then
      return modified
    end

    severity = trim(severity):upper()

    if severity == "I" or severity == "INF" or severity == "INFO" then
      record["severity"] = "INFO"
      modified = true
    elseif severity == "W" or severity == "WRN" or severity == "WARN" or severity == "WARNING" then
      record["severity"] = "WARN"
      modified = true
    elseif severity == "E" or severity == "ERR" or severity == "ERROR" or severity == "EROR" then
      record["severity"] = "ERR"
      modified = true
    elseif severity == "D" or severity == "DBG" or severity == "DEBUG" then
      record["severity"] = "DBG"
      modified = true
    elseif severity == "N" or severity == "NOTICE" then
      record["severity"] = "NOTICE"
      modified = true
    elseif severity == "F" or severity == "FATAL" then
      record["severity"] = "FATAL"
      modified = true
    end

    return modified
  end

  function trim(s)
    return (s:gsub("^%s*(.-)%s*$", "%1"))
  end

add_tag_to_record.lua: |-
  function add_tag_to_record(tag, timestamp, record)
    record["tag"] = tag
    return 1, timestamp, record
  end

kubernetes_label_map.json: |-
  {
    "kubernetes": {{ toJson .Values.lokiLabels.kubernetesLabels }} ,
    "severity": "severity",
    "job": "job"
  }

systemd_label_map.json: |-
{{ toJson .Values.lokiLabels.systemdLabels | indent 2 }}
{{- end -}}

{{- define "fluent-bit.config.name" -}}
fluent-bit-config-{{ include "fluent-bit.config.data" . | sha256sum | trunc 8 }}
{{- end }}
