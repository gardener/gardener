extensions:
  file_storage:
    directory: /var/log/otelcol
    create_directory: true
  bearertokenauth:
    filename: {{ .pathAuthToken }}

receivers:
  journald/journal:
    start_at: beginning
    storage: file_storage
    operators:
      - type: move
        from: body.SYSLOG_IDENTIFIER
        to: resource.unit
      - type: move
        from: body._HOSTNAME
        to: resource.nodename
      - type: retain
        fields:
          - body.MESSAGE

  filelog/pods:
    include: /var/log/pods/kube-system_*/*/*.log
    storage: file_storage
    include_file_path: true
    operators:
      - type: container
        format: containerd
        add_metadata_from_filepath: true

processors:
  batch:
    timeout: 10s

  k8sattributes:
    auth_type: "kubeConfig"
    context: "shoot"
    filter:
        namespace: kube-system
        labels:
          - key: resources.gardener.cloud/managed-by
            op: equals
            value: gardener
    pod_association:
      - sources:
          - from: resource_attributes
            name: k8s.pod.name

  resourcedetection/system:
    detectors: ["system"]
    system:
      hostname_sources: ["os"]

  filter/drop_localhost_journal:
    logs:
      exclude:
        match_type: strict
        resource_attributes:
          - key: _HOSTNAME
            value: localhost

  filter/keep_units_journal:
    logs:
      include:
        match_type: strict
        resource_attributes:
          - key: SYSLOG_IDENTIFIER
            value: kernel
          - key: _SYSTEMD_UNIT
            value: kubelet.service
          - key: _SYSTEMD_UNIT
            value: docker.service
          - key: _SYSTEMD_UNIT
            value: containerd.service
          - key: _SYSTEMD_UNIT
            value: gardener-node-agent.service

  filter/drop_units_combine:
    logs:
      exclude:
        match_type: strict
        resource_attributes:
          - key: SYSLOG_IDENTIFIER
            value: kernel
          - key: _SYSTEMD_UNIT
            value: kubelet.service
          - key: _SYSTEMD_UNIT
            value: docker.service
          - key: _SYSTEMD_UNIT
            value: containerd.service
          - key: _SYSTEMD_UNIT
            value: gardener-node-agent.service

  resource/journal:
    attributes:
      - action: insert
        key: origin
        value: systemd-journal
      - key: loki.resource.labels
        value: unit, nodename, origin
        action: insert
      - key: loki.format
        value: logfmt
        action: insert

  resource/pod_labels:
    attributes:
      - key: origin
        value: "shoot-system"
        action: insert
      - key: namespace_name
        value: "kube-system"
        action: insert
      - key: pod_name
        from_attribute: k8s.pod.name
        action: insert
      - key: container_name
        from_attribute: k8s.container.name
        action: insert
      - key: loki.resource.labels
        value: pod_name, container_name, origin, namespace_name, nodename, host.name
        action: insert
      - key: loki.format
        value: logfmt
        action: insert

exporters:
  loki:
    endpoint: {{ .clientURL }}
    auth:
      authenticator: bearertokenauth
    tls:
      ca_file: {{ .pathCACert }}

  debug:
    verbosity: detailed

service:
  extensions: [file_storage, bearertokenauth]
  pipelines:
    logs/journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/keep_units_journal, resource/journal, batch]
      exporters: [loki]
    logs/combine_journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/drop_units_combine, resource/journal, batch]
      exporters: [loki]
    logs/pods:
      receivers: [filelog/pods]
      processors: [k8sattributes, resourcedetection/system, resource/pod_labels, batch]
      exporters: [loki, debug]
