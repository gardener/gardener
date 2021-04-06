{{- define "fluent-bit.conf" }}
{{ if .Values.fluentBitConfigurationsOverwrites.service }}
{{ .Values.fluentBitConfigurationsOverwrites.service | indent 4 }}
{{ else }}
    [SERVICE]
        Flush           30
        Daemon          Off
        Log_Level       info
        Parsers_File    parsers.conf
        HTTP_Server     On
        HTTP_Listen     0.0.0.0
        HTTP_PORT       {{ .Values.ports.metrics }}
{{ end }}

    @INCLUDE input.conf
    @INCLUDE filter-kubernetes.conf
    @INCLUDE output.conf
{{- end }}
{{- define "input.conf" }}
{{ if .Values.fluentBitConfigurationsOverwrites.input }}
{{ .Values.fluentBitConfigurationsOverwrites.input | indent 4 }}
{{ else }}
    [INPUT]
        Name              tail
        Tag               kubernetes.*
        Path              /var/log/containers/*.log
        Exclude_Path      *_garden_fluent-bit-*.log,*_garden_loki-*.log
        Parser            docker
        DB                /var/log/flb_kube.db
        DB.sync           full
        read_from_head    true
        Skip_Long_Lines   On
        Mem_Buf_Limit     30MB
        Refresh_Interval  10
        Ignore_Older      1800s

    [INPUT]
        Name            systemd
        Tag             journald.docker
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=docker.service

    [INPUT]
        Name            systemd
        Tag             journald.kubelet
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=kubelet.service

    [INPUT]
        Name            systemd
        Tag             journald.containerd
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=containerd.service

    [INPUT]
        Name            systemd
        Tag             journald.cloud-config-downloader
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=cloud-config-downloader.service

    [INPUT]
        Name            systemd
        Tag             journald.docker-monitor
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=docker-monitor.service

    [INPUT]
        Name            systemd
        Tag             journald.containerd-monitor
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=containerd-monitor.service

    [INPUT]
        Name            systemd
        Tag             journald.kubelet-monitor
        Path            %%JOURNALD_PATH%%
        Read_From_Tail  True
        Systemd_Filter  _SYSTEMD_UNIT=kubelet-monitor.service
{{ end }}
{{- end }}
{{- define "output.conf" }}
{{ if .Values.fluentBitConfigurationsOverwrites.output }}
{{ .Values.fluentBitConfigurationsOverwrites.output | indent 4 }}
{{ else }}
    [Output]
        Name gardenerloki
        Match kubernetes.*
        Url http://loki.garden.svc:3100/loki/api/v1/push
        LogLevel info
        BatchWait 40s
        BatchSize 30720
        Labels {test="fluent-bit-go"}
        LineFormat json
        SortByTimestamp true
        DropSingleKey false
        AutoKubernetesLabels false
        LabelSelector gardener.cloud/role:shoot
        RemoveKeys kubernetes,stream,time,tag
        LabelMapPath /fluent-bit/etc/kubernetes_label_map.json
        DynamicHostPath {"kubernetes": {"namespace_name": "namespace"}}
        DynamicHostPrefix http://loki.
        DynamicHostSuffix .svc:3100/loki/api/v1/push
        DynamicHostRegex ^shoot-
        MaxRetries 3
        Timeout 10s
        MinBackoff 30s
        Buffer true
        BufferType dque
        QueueDir  /fluent-bit/buffers/operator
        QueueSegmentSize 300
        QueueSync normal
        QueueName gardener-kubernetes-operator
        FallbackToTagWhenMetadataIsMissing true
        TagKey tag
        DropLogEntryWithoutK8sMetadata true
        SendDeletedClustersLogsToDefaultClient true
        CleanExpiredClientsPeriod 1h
        ControllerSyncTimeout 120s
        NumberOfBatchIDs 5
        TenantID operator
    
    [Output]
        Name gardenerloki
        Match {{ .Values.exposedComponentsTagPrefix }}.kubernetes.*
        Url http://loki.garden.svc:3100/loki/api/v1/push
        LogLevel info
        BatchWait 40s
        BatchSize 30720
        Labels {test="fluent-bit-go", lang="Golang"}
        LineFormat json
        SortByTimestamp true
        DropSingleKey false
        AutoKubernetesLabels true
        LabelSelector gardener.cloud/role:shoot
        RemoveKeys kubernetes,stream,type,time,tag
        LabelMapPath /fluent-bit/etc/kubernetes_label_map.json
        DynamicHostPath {"kubernetes": {"namespace_name": "namespace"}}
        DynamicHostPrefix http://loki.
        DynamicHostSuffix .svc:3100/loki/api/v1/push
        DynamicHostRegex ^shoot-
        MaxRetries 3
        Timeout 10s
        MinBackoff 30s
        Buffer true
        BufferType dque
        QueueDir  /fluent-bit/buffers/user
        QueueSegmentSize 300
        QueueSync normal
        QueueName gardener-kubernetes-user
        FallbackToTagWhenMetadataIsMissing true
        SendDeletedClustersLogsToDefaultClient true
        TagKey tag
        DropLogEntryWithoutK8sMetadata true
        ControllerSyncTimeout 120s
        NumberOfBatchIDs 5
        TenantID user

    [Output]
        Name gardenerloki
        Match journald.*
        Url http://loki.garden.svc:3100/loki/api/v1/push
        LogLevel info
        BatchWait 60s
        BatchSize 30720
        Labels {test="fluent-bit-go"}
        LineFormat json
        SortByTimestamp true
        DropSingleKey false
        RemoveKeys kubernetes,stream,hostname,unit
        LabelMapPath /fluent-bit/etc/systemd_label_map.json
        MaxRetries 3
        Timeout 10s
        MinBackoff 30s
        Buffer true
        BufferType dque
        QueueDir  /fluent-bit/buffers
        QueueSegmentSize 300
        QueueSync normal
        QueueName gardener-journald
        NumberOfBatchIDs 5
{{ end }}
{{- end }}
