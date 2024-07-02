---
weight: 3
description: Defining the maintenance time window, enabling automatic version updates, confine reconcilations only to happen during maintenance, adding an additional maintenance operation and list of special operations during maintenance
---

# Shoot Maintenance

Shoots configure a maintenance time window in which Gardener performs certain operations that may restart the control plane, roll out the nodes, result in higher network traffic, etc. A summary of what was changed in the last maintenance time window in shoot specification is kept in the shoot status `.status.lastMaintenance` field.

This document outlines what happens during a shoot maintenance.

## Time Window

Via the `.spec.maintenance.timeWindow` field in the shoot specification, end-users can configure the time window in which maintenance operations are executed.
Gardener runs one maintenance operation per day in this time window:

```yaml
spec:
  maintenance:
    timeWindow:
      begin: 220000+0100
      end: 230000+0100
```

The offset (`+0100`) is considered with respect to UTC time.
The minimum time window is `30m` and the maximum is `6h`.

⚠️ Please note that there is no guarantee that a maintenance operation that, e.g., starts a node roll-out will finish *within* the time window.
Especially for large clusters, it may take several hours until a graceful rolling update of the worker nodes succeeds (also depending on the workload and the configured pod disruption budgets/termination grace periods).

Internally, Gardener is subtracting `15m` from the end of the time window to (best-effort) try to finish the maintenance until the end is reached, however, this might not work in all cases.

If you don't specify a time window, then Gardener will randomly compute it.
You can change it later, of course.

## Automatic Version Updates

The `.spec.maintenance.autoUpdate` field in the shoot specification allows you to control how/whether automatic updates of Kubernetes patch and machine image versions are performed.
Machine image versions are updated per worker pool.

```yaml
spec:
  maintenance:
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
```

During the daily maintenance, the Gardener Controller Manager updates the Shoot's Kubernetes and machine image version if any of the following criteria applies:
 - There is a higher version available and the Shoot opted-in for automatic version updates.
 - The currently used version is `expired`.

The target version for machine image upgrades is controlled by the `updateStrategy` field for the machine image in the CloudProfile. Allowed update strategies are `patch`, `minor` and `major`.

Gardener (gardener-controller-manager) populates the `lastMaintenance` field in the Shoot status with the maintenance results.

```yaml
Last Maintenance:
    Description:     "All maintenance operations successful. Control Plane: Updated Kubernetes version from 1.26.4 to 1.27.1. Reason: Kubernetes version expired - force update required"
    State:           Succeeded
    Triggered Time:  2023-07-28T09:07:27Z
```

Additionally, Gardener creates events with the type `MachineImageVersionMaintenance` or `KubernetesVersionMaintenance` on the Shoot describing the action performed during maintenance, including the reason why an update has been triggered.

```text
LAST SEEN   TYPE      REASON                           OBJECT          MESSAGE
30m         Normal    MachineImageVersionMaintenance   shoot/local     Worker pool "local": Updated image from 'gardenlinux' version 'xy' to version 'abc'. Reason: Automatic update of the machine image version is configured (image update strategy: major).

30m         Normal    KubernetesVersionMaintenance     shoot/local     Control Plane: Updated Kubernetes version from "1.26.4" to "1.27.1". Reason: Kubernetes version expired - force update required.

15m         Normal    KubernetesVersionMaintenance     shoot/local     Worker pool "local": Updated Kubernetes version '1.26.3' to version '1.27.1'. Reason: Kubernetes version expired - force update required.
```

If at least one maintenance operation fails, the `lastMaintenance` field in the Shoot status is set to `Failed`:

```yaml
Last Maintenance:
  Description:     "(1/2) maintenance operations successful: Control Plane: Updated Kubernetes version from 1.26.4 to 1.27.1. Reason: Kubernetes version expired - force update required, Worker pool x: 'gardenlinux' machine image version maintenance failed. Reason for update: machine image version expired"
  FailureReason:   "Worker pool x: either the machine image 'gardenlinux' is reaching end of life and migration to another machine image is required or there is a misconfiguration in the CloudProfile."
  State:           Failed
  Triggered Time:  2023-07-28T09:07:27Z
```

Please refer to the [Shoot Kubernetes and Operating System Versioning in Gardener](../shoot_updates_and_upgrades/shoot_versions.md) topic for more information about Kubernetes and machine image versions in Gardener.

## When are Clusters Reconciled

Gardener administrators/operators can configure the gardenlet in a way that it only reconciles shoot clusters during their maintenance time windows.
This behaviour is not controllable by end-users but might make sense for large Gardener installations.
Concretely, your shoot will be reconciled regularly during its maintenance time window.
Outside of the maintenance time window it will only reconcile if you change the specification or if you explicitly trigger it, see also [Trigger Shoot Operations](../operating_through_annotations/shoot_operations.md).

### Confine Specification Changes/Updates Roll Out

Via the `.spec.maintenance.confineSpecUpdateRollout` field you can control whether you want to make Gardener roll out changes/updates to your shoot specification only during the maintenance time window.
It is `false` by default, i.e., any change to your shoot specification triggers a reconciliation (even outside of the maintenance time window).
This is helpful if you want to update your shoot but don't want the changes to be applied immediately. One example use-case would be a Kubernetes version upgrade that you want to roll out during the maintenance time window.
Any update to the specification will not increase the `.metadata.generation` of the `Shoot`, which is something you should be aware of.
Also, even if Gardener administrators/operators have not enabled the "reconciliation in maintenance time window only" configuration (as mentioned above), then your shoot will only reconcile in the maintenance time window.
The reason is that Gardener cannot differentiate between create/update/reconcile operations.

⚠️ If `confineSpecUpdateRollout=true`, please note that if you change the maintenance time window itself, then it will only be effective after the upcoming maintenance.

⚠️ As exceptions to the above rules, [manually triggered reconciliations](../operating_through_annotations/shoot_operations.md#immediate-reconciliation) and changes to the `.spec.hibernation.enabled` field trigger immediate rollouts.
I.e., if you hibernate or wake-up your shoot, or you explicitly tell Gardener to reconcile your shoot, then Gardener gets active right away.

## Additional Operation

In case you would like to perform a [shoot credential rotation](../operating_through_annotations/shoot_operations.md#credentials-rotation-operations) or a `reconcile` operation during your maintenance time window, you can annotate the `Shoot` with

```
maintenance.gardener.cloud/operation=<operation>
```

This will execute the specified `<operation>` during the next maintenance reconciliation.
Note that Gardener will remove this annotation after it has been performed in the maintenance reconciliation.

> ⚠️ This is skipped when the `Shoot`'s `.status.lastOperation.state=Failed`. Make sure to [retry](../operating_through_annotations/shoot_operations.md#retry-failed-reconciliation) your shoot reconciliation beforehand.

## Special Operations During Maintenance

The shoot maintenance controller triggers special operations that are performed as part of the shoot reconciliation.

### `Infrastructure` and `DNSRecord` Reconciliation

The reconciliation of the `Infrastructure` and `DNSRecord` extension resources is only demanded during the shoot's maintenance time window.
The rationale behind it is to prevent sending too many requests against the cloud provider APIs, especially on large landscapes or if a user has many shoot clusters in the same cloud provider account.

### Restart Control Plane Controllers

Gardener operators can make Gardener restart/delete certain control plane pods during a shoot maintenance.
This feature helps to automatically solve service denials of controllers due to stale caches, dead-locks or starving routines.

Please note that these are exceptional cases but they are observed from time to time.
Gardener, for example, takes this precautionary measure for `kube-controller-manager` pods.

See [Shoot Maintenance](../../extensions/shoot-maintenance.md) to see how extension developers can extend this behaviour.

### Restart Some Core Addons

Gardener operators can make Gardener restart some core addons (at the moment only CoreDNS) during a shoot maintenance.

CoreDNS benefits from this feature as it automatically solve problems with clients stuck to single replica of the deployment and thus overloading it.
Please note that these are exceptional cases but they are observed from time to time.
