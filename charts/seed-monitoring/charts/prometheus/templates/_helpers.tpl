{{- define "prometheus.kubelet-shoot" }}
      honor_labels: false
      scheme: https

      tls_config:
        # This is needed because the kubelets' certificates are not are generated
        # for a specific pod IP
        insecure_skip_verify: true
        cert_file: /etc/prometheus/seed/prometheus.crt
        key_file: /etc/prometheus/seed/prometheus.key

      kubernetes_sd_configs:
      - role: node
        api_server: kube-apiserver
        tls_config:
          ca_file: /etc/prometheus/seed/ca.crt
          cert_file: /etc/prometheus/seed/prometheus.crt
          key_file: /etc/prometheus/seed/prometheus.key

      relabel_configs:
      - target_label: __metrics_path__
        replacement: /{{.Path}}
      - source_labels: [__meta_kubernetes_node_address_InternalIP]
        target_label: instance
      - action: labelmap
        regex: __meta_kubernetes_node_label_(.+)
      - target_label: type
        replacement: shoot
      - target_label: job
        replacement: {{.JobName}}

      # get system services
      metric_relabel_configs:
      - source_labels: [id]
        action: replace
        regex: '^/system\.slice/(.+)\.service$'
        target_label: systemd_service_name
        replacement: '${1}'
      # We want to keep only metrics in kube-system namespace
      - source_labels: [namespace]
        action: keep
        regex: kube-system
{{- end }}


{{- define "prometheus.kubelet-seed" }}
      honor_labels: false
      scheme: https

      tls_config:
        # ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        # This is needed because the kubelets' certificates are not are generated
        # for a specific pod IP
        insecure_skip_verify: true
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

      kubernetes_sd_configs:
      - role: node
      relabel_configs:
      - target_label: __metrics_path__
        replacement: /{{.Path}}
      - source_labels: [__meta_kubernetes_node_address_InternalIP]
        target_label: instance
      - action: labelmap
        regex: __meta_kubernetes_node_label_(.+)
      - target_label: type
        replacement: seed
      - target_label: job
        replacement: {{.JobName}}

      # Here we do the actual multi-tenancy - we only get one in specific namespace
      metric_relabel_configs:
      - source_labels: [namespace]
        action: keep
        regex: {{ .Namespace }}
      # The job name will only be applied to the metrics, not to the targets
      # in the prometheus UI!!!
      # we make the shoot's pods in the shoot's namepsace to apear in as its in the kube-system
      - target_label: namespace
        replacement: kube-system

      # get system services
      - source_labels: [id]
        action: replace
        regex: '^/system\.slice/(.+)\.service$'
        target_label: systemd_service_name
        replacement: '${1}'
      {{- end }}

      {{- define "prometheus.service-endpoints.relabel-config" }}
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
        action: replace
        target_label: __scheme__
        regex: (https?)
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
        action: replace
        target_label: __address__
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
      - action: labelmap
        regex: __meta_kubernetes_service_label_(.+)
      - source_labels: [__meta_kubernetes_service_name]
        action: replace
        target_label: job
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_name]
        action: replace
        target_label: job
        regex: (.+)
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
{{- end}}