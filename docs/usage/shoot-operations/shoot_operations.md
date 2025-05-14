---
title: Trigger Shoot Operations Through Annotations
---

# Trigger Shoot Operations Through Annotations

You can trigger a few explicit operations by annotating the `Shoot` with an operation annotation.
This might allow you to induct certain behavior without the need to change the `Shoot` specification.
Some of the operations can also not be caused by changing something in the shoot specification because they can't properly be reflected here.
Note that once the triggered operation is considered by the controllers, the annotation will be automatically removed and you have to add it each time you want to trigger the operation.

Please note: If `.spec.maintenance.confineSpecUpdateRollout=true`, then the only way to trigger a shoot reconciliation is by setting the `reconcile` operation, see below.

## Immediate Reconciliation

Annotate the shoot with `gardener.cloud/operation=reconcile` to make the `gardenlet` start a reconciliation operation without changing the shoot spec and possibly without being in its maintenance time window:

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=reconcile
```

## Immediate Maintenance

Annotate the shoot with `gardener.cloud/operation=maintain` to make the `gardener-controller-manager` start maintaining your shoot immediately (possibly without being in its maintenance time window).
If no reconciliation starts, then nothing needs to be maintained:

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=maintain
```

## Retry Failed Reconciliation

Annotate the shoot with `gardener.cloud/operation=retry` to make the `gardenlet` start a new reconciliation loop on a failed shoot.
Failed shoots are only reconciled again if a new Gardener version is deployed, the shoot specification is changed or this annotation is set:

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=retry
```

## Force-update a worker pool with InPlace update strategy

Annotate the shoot with `gardener.cloud/operation=force-in-place-update` to force an update for worker pools using the update strategy `AutoInPlaceUpdate` or `ManualInPlaceUpdate`. Without this annotation, any subsequent updates to the same worker pool are denied until the `Shoot` has been successfully reconciled following the current in-place update.


```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=force-in-place-update
```

## Credentials Rotation Operations

Please consult [Credentials Rotation for Shoot Clusters](shoot_credentials_rotation.md) for more information.

## Restart `systemd` Services on Particular Worker Nodes

It is possible to make Gardener restart particular systemd services on your shoot worker nodes if needed.
The annotation is not set on the `Shoot` resource but directly on the `Node` object you want to target.
For example, the following will restart both the `kubelet` and the `containerd` services:

```bash
kubectl annotate node <node-name> worker.gardener.cloud/restart-systemd-services=kubelet,containerd
```

It may take up to a minute until the service is restarted.
The annotation will be removed from the `Node` object after all specified systemd services have been restarted.
It will also be removed even if the restart of one or more services failed.

> ℹ️ In the example mentioned above, you could additionally verify when/whether the kubelet restarted by using `kubectl describe node <node-name>` and looking for such a `Starting kubelet` event.

## Force Deletion

When a Shoot fails to be deleted normally, users can force-delete the Shoot by meeting the following conditions:

- Shoot has a deletion timestamp.
- Shoot status contains at least one of the following [ErrorCodes](../shoot/shoot_status.md#error-codes):
  - `ERR_CLEANUP_CLUSTER_RESOURCES`
  - `ERR_CONFIGURATION_PROBLEM`
  - `ERR_INFRA_DEPENDENCIES`
  - `ERR_INFRA_UNAUTHENTICATED`
  - `ERR_INFRA_UNAUTHORIZED`

If the above conditions are satisfied, you can annotate the Shoot with `confirmation.gardener.cloud/force-deletion=true`, and Gardener will cleanup the Shoot controlplane and the Shoot metadata.

> :warning: You **MUST** ensure that all the resources created in the IaaS account are cleaned up to prevent orphaned resources. Gardener will **NOT** delete any resources in the underlying infrastructure account. Hence, use this annotation at your own risk and only if you are fully aware of these consequences.
