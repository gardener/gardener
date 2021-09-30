# Operator Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
|ApiServerUnreachableViaKubernetesService|critical|shoot|`The Api server has been unreachable for 3 minutes via the kubernetes service in the shoot.`|
|KubeletTooManyOpenFileDescriptorsSeed|critical|seed|`Seed-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubePersistentVolumeUsageCritical|critical|seed|`The PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} is only {{ printf "%0.2f" $value }}% free.`|
|KubePersistentVolumeFullInFourDays|warning|seed|`Based on recent sampling, the PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} is expected to fill up within four days. Currently {{ printf "%0.2f" $value }}% is available.`|
|KubePodPendingControlPlane|warning|seed|`Pod {{ $labels.pod }} is stuck in "Pending" state for more than 30 minutes.`|
|KubePodNotReadyControlPlane|warning||`Pod {{ $labels.pod }} is not ready for more than 30 minutes.`|
|KubeStateMetricsShootDown|info|seed|`There are no running kube-state-metric pods for the shoot cluster. No kubernetes resource metrics can be scraped.`|
|KubeStateMetricsSeedDown|critical|seed|`There are no running kube-state-metric pods for the seed cluster. No kubernetes resource metrics can be scraped.`|
|NoWorkerNodes|blocker||`There are no worker nodes in the cluster or all of the worker nodes in the cluster are not schedulable.`|
|PrometheusCantScrape|warning|seed|`Prometheus failed to scrape metrics. Instance {{ $labels.instance }}, job {{ $labels.job }}.`|
|PrometheusConfigurationFailure|warning|seed|`Latest Prometheus configuration is broken and Prometheus is using the previous one.`|
|VPNShootNoPods|critical|shoot|`vpn-shoot deployment in Shoot cluster has 0 available pods. VPN won't work.`|
|VPNProbeAPIServerProxyFailed|critical|shoot|`The API Server proxy functionality is not working. Probably the vpn connection from an API Server pod to the vpn-shoot endpoint on the Shoot workers does not work.`|
