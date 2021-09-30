# User Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
|ApiServerUnreachableViaKubernetesService|critical|shoot|`The Api server has been unreachable for 3 minutes via the kubernetes service in the shoot.`|
|KubeKubeletNodeDown|warning|shoot|`The kubelet {{ $labels.instance }} has been unavailable/unreachable for more than 1 hour. Workloads on the affected node may not be schedulable.`|
|KubeletTooManyOpenFileDescriptorsShoot|warning|shoot|`Shoot-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubeletTooManyOpenFileDescriptorsShoot|critical|shoot|`Shoot-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubePodPendingShoot|warning|shoot|`Pod {{ $labels.pod }} is stuck in "Pending" state for more than 1 hour.`|
|KubePodNotReadyShoot|warning|shoot|`Pod {{ $labels.pod }} is not ready for more than 1 hour.`|
|NoWorkerNodes|blocker||`There are no worker nodes in the cluster or all of the worker nodes in the cluster are not schedulable.`|
|NodeExporterDown|warning|shoot|`The NodeExporter has been down or unreachable from Prometheus for more than 1 hour.`|
|K8SNodeOutOfDisk|critical|shoot|`Node {{ $labels.node }} has run out of disk space.`|
|K8SNodeMemoryPressure|warning|shoot|`Node {{ $labels.node }} is under memory pressure.`|
|K8SNodeDiskPressure|warning|shoot|`Node {{ $labels.node }} is under disk pressure`|
|VMRootfsFull|critical|shoot|`Root filesystem device on instance {{ $labels.instance }} is almost full.`|
|VMConntrackTableFull|critical|shoot|`The nf_conntrack table is {{ $value }}% full.`|
|VPNProbeAPIServerProxyFailed|critical|shoot|`The API Server proxy functionality is not working. Probably the vpn connection from an API Server pod to the vpn-shoot endpoint on the Shoot workers does not work.`|
