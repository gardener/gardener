# User Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
|KubeKubeletNodeDown|warning|shoot|`The kubelet {{ $labels.instance }} has been unavailable/unreachable for more than 1 hour. Workloads on the affected node may not be schedulable.`|
|KubeletTooManyOpenFileDescriptorsShoot|warning|shoot|`Shoot-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubeletTooManyOpenFileDescriptorsShoot|critical|shoot|`Shoot-kubelet ({{ $labels.kubernetes_io_hostname }}) is using {{ $value }}% of the available file/socket descriptors. Kubelet could be under heavy load.`|
|KubePodPendingShoot|warning|shoot|`Pod {{ $labels.pod }} is stuck in "Pending" state for more than 1 hour.`|
|KubePodNotReadyShoot|warning|shoot|`Pod {{ $labels.pod }} is not ready for more than 1 hour.`|
