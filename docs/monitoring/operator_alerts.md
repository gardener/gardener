# Operator Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
|ApiServerUnreachableViaKubernetesService|critical|shoot|`The Api server has been unreachable for 3 minutes via the kubernetes service in the shoot.`|
|CoreDNSDown|critical|shoot|`CoreDNS could not be found. Cluster DNS resolution will not work.`|
|ApiServerNotReachable|blocker|seed|`API server not reachable via external endpoint: {{ $labels.instance }}.`|
|KubeApiserverDown|blocker|seed|`All API server replicas are down/unreachable, or all API server could not be found.`|
|KubeApiServerTooManyAuditlogFailures|warning|seed|`The API servers cumulative failure rate in logging audit events is greater than 2%.`|
|KubeControllerManagerDown|critical|seed|`Deployments and replication controllers are not making progress.`|
|KubeEtcdMainDown|blocker|seed|`Etcd3 cluster main is unavailable or cannot be scraped. As long as etcd3 main is down the cluster is unreachable.`|
|KubeEtcdEventsDown|critical|seed|`Etcd3 cluster events is unavailable or cannot be scraped. Cluster events cannot be collected.`|
|KubeEtcd3MainNoLeader|critical|seed|`Etcd3 main has no leader. No communication with etcd main possible. Apiserver is read only.`|
|KubeEtcd3EventsNoLeader|critical|seed|`Etcd3 events has no leader. No communication with etcd events possible. New cluster events cannot be collected. Events can only be read.`|
|KubeEtcd3HighNumberOfFailedProposals|warning|seed|`Etcd3 pod {{ $labels.pod }} has seen {{ $value }} proposal failures within the last hour.`|
|KubeEtcd3DbSizeLimitApproaching|warning|seed|`Etcd3 {{ $labels.role }} DB size is approaching its current practical limit of 8GB. Etcd quota might need to be increased.`|
|KubeEtcd3DbSizeLimitCrossed|critical|seed|`Etcd3 {{ $labels.role }} DB size has crossed its current practical limit of 8GB. Etcd quota must be increased to allow updates.`|
|KubeEtcdDeltaBackupFailed|critical|seed|`No delta snapshot for the past at least 30 minutes.`|
|KubeEtcdFullBackupFailed|critical|seed|`No full snapshot taken in the past day.`|
|KubeEtcdRestorationFailed|critical|seed|`Etcd data restoration was triggered, but has failed.`|
|KubeletTooManyOpenFileDescriptorsSeed|critical|seed|`Seed-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubePersistentVolumeUsageCritical|critical|seed|`The PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} is only {{ printf "%0.2f" $value }}% free.`|
|KubePersistentVolumeFullInFourDays|warning|seed|`Based on recent sampling, the PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} is expected to fill up within four days. Currently {{ printf "%0.2f" $value }}% is available.`|
|KubePodPendingControlPlane|warning|seed|`Pod {{ $labels.pod }} is stuck in "Pending" state for more than 30 minutes.`|
|KubePodNotReadyControlPlane|warning||`Pod {{ $labels.pod }} is not ready for more than 30 minutes.`|
|KubeSchedulerDown|critical|seed|`New pods are not being assigned to nodes.`|
|KubeStateMetricsShootDown|info|seed|`There are no running kube-state-metric pods for the shoot cluster. No kubernetes resource metrics can be scraped.`|
|KubeStateMetricsSeedDown|critical|seed|`There are no running kube-state-metric pods for the seed cluster. No kubernetes resource metrics can be scraped.`|
|NoWorkerNodes|blocker||`There are no worker nodes in the cluster or all of the worker nodes in the cluster are not schedulable.`|
|PrometheusCantScrape|warning|seed|`Prometheus failed to scrape metrics. Instance {{ $labels.instance }}, job {{ $labels.job }}.`|
|PrometheusConfigurationFailure|warning|seed|`Latest Prometheus configuration is broken and Prometheus is using the previous one.`|
|VPNShootNoPods|critical|shoot|`vpn-shoot deployment in Shoot cluster has 0 available pods. VPN won't work.`|
|VPNProbeAPIServerProxyFailed|critical|shoot|`The API Server proxy functionality is not working. Probably the vpn connection from an API Server pod to the vpn-shoot endpoint on the Shoot workers does not work.`|
