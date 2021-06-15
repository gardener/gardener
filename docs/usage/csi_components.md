# (Custom) CSI Components

Some provider extensions for Gardener are using CSI components to manage persistent volumes in the shoot clusters.
Additionally, most of the provider extensions are even deploying controllers for taking volume snapshots (CSI snapshotter).

End-users can deploy their own CSI components and controllers into shoot clusters.
In such situations, there are multiple controllers acting on the `VolumeSnapshot` custom resources (each responsible for those instances associated with their respective driver provisioner types).

## Recommendations

Custom CSI components are typically regular `Deployment`s running in the shoot clusters.

**Please label them with the `shoot.gardener.cloud/no-cleanup=true` label.**

## Background Information

When a shoot cluster is deleted, Gardener deletes most Kubernetes (`Deployment`s, `DaemonSet`s, `StatefulSet`s, etc.) resources, i.e., CSI components not having above mentioned label will be deleted immediately.

If the CSI components (and all resources associated with them) managed by end-users aren't properly removed by the end-users before shoot deletion is triggered then the cluster deletion might cause problems and get stuck.

This results in `VolumeSnapshot` resources still having finalizers that will never be cleaned up.
Consequently, manual intervention is required to clean them up before the cluster deletion can continue.
