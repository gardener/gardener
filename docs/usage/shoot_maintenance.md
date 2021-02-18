# Shoot Maintenance

Shoots configure a maintenance time window in which Gardener performs certain operations that may restart the control plane, roll out the nodes, result in higher network traffic, etc.
This document outlines what happens during a shoot maintenance.

## Time Window

Via the `.spec.maintenance.timeWindow` field in the shoot specification end-users can configure the time window in which maintenance operations are executed.
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

⚠️ Please note that there is no guarantee that a maintenance operation that e.g. starts a node roll-out will finish *within* the time window.
Especially for large clusters it may take several hours until a graceful rolling update of the worker nodes succeeds (also depending on the workload and the configured pod disruption budgets/termination grace periods).

Internally, Gardener is subtracting `15m` from the end of the time window to (best-effort) try to finish the maintenance until the end is reached, however, it might not work in all cases.

If you don't specify a time window then Gardener will randomly compute it.
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

 - there is a higher version available and the Shoot opted-in for automatic version updates
 - the currently used version is `expired`

Gardener creates events with type `MaintenanceDone` on the Shoot describing the action performed during maintenance including the reason why an update has been triggered.

```yaml
MaintenanceDone  Updated image of worker-pool 'coreos-xy' from 'coreos' version 'xy' to version 'abc'. Reason: AutoUpdate of MachineImage configured.
MaintenanceDone  Updated Kubernetes version '0.0.1' to version '0.0.5'. This is an increase in the patch level. Reason: AutoUpdate of Kubernetes version configured.
MaintenanceDone  Updated Kubernetes version '0.0.5' to version '0.1.5'. This is an increase in the minor level. Reason: Kubernetes version expired - force update required.
```

Please refer to [this document](./shoot_versions.md) for more information about Kubernetes and machine image versions in Gardener.

## Cluster Reconciliation

Gardener administrators/operators can configure the Gardenlet in a way that it only reconciles shoot clusters during their maintenance time windows.
This behaviour is not controllable by end-users but might make sense for large Gardener installations.
Concretely, your shoot will be reconciled regularly during its maintenance time window.
Outside of the maintenance time window it will only reconcile if you change the specification or if you explicitly trigger it, see also [this document](shoot_operations.md).

## Confine Specification Changes/Updates Roll Out

Via the `.spec.maintenance.confineSpecUpdateRollout` field you can control whether you want to make Gardener roll out changes/updates to your shoot specification only during the maintenance time window.
It is `false` by default, i.e., any change to your shoot specification triggers a reconciliation (even outside of the maintenance time window).
This is helpful if you want to update your shoot but don't want the changes to be applied immediately. One example use-case would be a Kubernetes version upgrade that you want to roll out during the maintenance time window.
Any update to the specification will not increase the `.metadata.generation` of the `Shoot` which is something you should be aware of.
Also, even if Gardener administrators/operators have not enabled the "reconciliation in maintenance time window only" configuration (as mentioned above) then your shoot will only reconcile in the maintenance time window.
The reason is that Gardener cannot differentiate between create/update/reconcile operations.

⚠️  If `confineSpecUpdateRollout=true`, please note that if you change the maintenance time window itself then it will only be effective after the upcoming maintenance.

⚠️  There is one exceptional change in the shoot specification that triggers an immediate roll out which is changes to the `.spec.hibernation.enabled` field.
If you hibernate or wake-up your shoot then Gardener gets active right away.

## Special Operations During Maintenance

The shoot maintenance controller triggers special operations that are performed as part of the shoot reconciliation.

### Infrastructure Reconciliation

The reconciliation of the `Infrastructure` extension resource is only demanded during the shoot's maintenance time window.
The rationale behind it is to prevent the Gardenlets from sending too many requests against the cloud provider APIs, especially if a user has many shoot clusters in the same cloud provider account.

### Restart Control Plane Controllers

Gardener operators can make Gardener restart/delete certain control plane pods during a shoot maintenance.
This feature helps to automatically solve service denials of controllers due to stale caches, dead-locks or starving routines.

Please note that these are exceptional cases but they are observed from time to time.
Gardener, for example, takes this precautionary measure for `kube-controller-manager` pods.

See [this document](../extensions/shoot-maintenance.md) to see how extension developers can extend this behaviour.

### Restart Some Core Addons

Gardener operators can make Gardener restart some core addons, at the moment only CoreDNS, during a shoot maintenance.

CoreDNS benefits from this feature as it automatically solve problems with clients stuck to single replica of the deployment and thus overloading it.
Please note that these are exceptional cases but they are observed from time to time.
