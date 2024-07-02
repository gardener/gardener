---
title: Necessary Labeling for Custom CSI Components
weight: 2
---

# (Custom) CSI Components

Some provider extensions for Gardener are using CSI components to manage persistent volumes in the shoot clusters.
Additionally, most of the provider extensions are deploying controllers for taking volume snapshots (CSI snapshotter).

End-users can deploy their own CSI components and controllers into shoot clusters.
In such situations, there are multiple controllers acting on the `VolumeSnapshot` custom resources (each responsible for those instances associated with their respective driver provisioner types).

However, this might lead to operational conflicts that cannot be overcome by Gardener alone.
Concretely, Gardener cannot know which custom CSI components were installed by end-users which can lead to issues, especially during shoot cluster deletion.
You can add a label to your custom CSI components indicating that Gardener should not try to remove them during shoot cluster deletion. This means you have to take care of the lifecycle for these components yourself!

## Recommendations

Custom CSI components are typically regular `Deployment`s running in the shoot clusters.

**Please label them with the `shoot.gardener.cloud/no-cleanup=true` label.**

## Background Information

When a shoot cluster is deleted, Gardener deletes most Kubernetes resources (`Deployment`s, `DaemonSet`s, `StatefulSet`s, etc.). Gardener will also try to delete CSI components if they are not marked with the above mentioned label.

This can result in `VolumeSnapshot` resources leaking in the cloud providers and manual cleanup will be required.
