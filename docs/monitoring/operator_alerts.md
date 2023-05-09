# Operator Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
|ApiServerUnreachableViaKubernetesService|critical|shoot|`The Api server has been unreachable for 15 minutes via the kubernetes service in the shoot.`|
|KubeletTooManyOpenFileDescriptorsSeed|critical|seed|`Seed-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubePersistentVolumeUsageCritical|critical|seed|`The PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} is only {{ printf "%0.2f" $value }}% free.`|
|KubePersistentVolumeFullInFourDays|warning|seed|`Based on recent sampling, the PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} is expected to fill up within four days. Currently {{ printf "%0.2f" $value }}% is available.`|
|KubePodPendingControlPlane|warning|seed|`Pod {{ $labels.pod }} is stuck in "Pending" state for more than 30 minutes.`|
|KubePodNotReadyControlPlane|warning||`Pod {{ $labels.pod }} is not ready for more than 30 minutes.`|
|VPNProbeAPIServerProxyFailed|critical|shoot|`The API Server proxy functionality is not working. Probably the vpn connection from an API Server pod to the vpn-shoot endpoint on the Shoot workers does not work.`|
|PrometheusCantScrape|warning|seed|`Prometheus failed to scrape metrics. Instance {{ $labels.instance }}, job {{ $labels.job }}.`|
|PrometheusConfigurationFailure|warning|seed|`Latest Prometheus configuration is broken and Prometheus is using the previous one.`|
