# (Custom) CSI Components

Some provider extensions for Gardener are using CSI components to manage persistent volumes in the shoot clusters.
Additionally, most of the provider extensions are deploying controllers for taking volume snapshots (CSI snapshotter).

End-users can deploy their own CSI components and controllers into shoot clusters.
In such situations, there are multiple controllers acting on the `VolumeSnapshot` custom resources (each responsible for those instances associated with their respective driver provisioner types).

However, this might lead to operational conflicts that cannot be overcome by Gardener alone.
Concretely, Gardener cannot know which custom CSI components were installed by end-users which can lead to issues, especially during shoot cluster deletion.

## Recommendations

Custom CSI components are typically regular `Deployment`s running in the shoot clusters.

**Please label them with the `shoot.gardener.cloud/no-cleanup=true` label.**

## Background Information

When a shoot cluster is deleted, Gardener deletes most Kubernetes resources (`Deployment`s, `DaemonSet`s, `StatefulSet`s, etc.). Gardener will also try to delete CSI components if they are not marked with the above mentioned label.


This results in `VolumeSnapshot` resources still having finalizers that will never be cleaned up.
Consequently, manual intervention is required to clean them up before the cluster deletion can continue.
