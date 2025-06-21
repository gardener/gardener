---
title: Shoot Updates and Upgrades
---

# Shoot Updates and Upgrades

This document describes what happens during shoot updates (changes incorporated in a newly deployed Gardener version) and during shoot upgrades (changes for version controllable by end-users).

## Updates

Updates to all aspects of the shoot cluster happen when the gardenlet reconciles the `Shoot` resource.

### When are Reconciliations Triggered

Generally, when you change the specification of your `Shoot` the reconciliation will start immediately, potentially updating your cluster.
Please note that you can also confine the reconciliation triggered due to your specification updates to the cluster's maintenance time window. Please find more information in [Confine Specification Changes/Updates Roll Out](../shoot/shoot_maintenance.md#confine-specification-changesupdates-roll-out).

You can also annotate your shoot with special operation annotations (for more information, see [Trigger Shoot Operations](shoot_operations.md)), which will cause the reconciliation to start due to your actions.

There is also an automatic reconciliation by Gardener.
The period, i.e., how often it is performed, depends on the configuration of the Gardener administrators/operators.
In some Gardener installations the operators might enable "reconciliation in maintenance time window only" (for more information, see [Cluster Reconciliation](../shoot/shoot_maintenance.md#cluster-reconciliation)), which will result in at least one reconciliation during the time configured in the `Shoot`'s `.spec.maintenance.timeWindow` field.

### Which Updates are Applied

As end-users can only control the `Shoot` resource's specification but not the used Gardener version, they don't have any influence on which of the updates are rolled out (other than those settings configurable in the `Shoot`).
A Gardener operator can deploy a new Gardener version at any point in time.
Any subsequent reconciliation of `Shoot`s will update them by rolling out the changes incorporated in this new Gardener version.

Some examples for such shoot updates are:

* Add a new/remove an old component to/from the shoot's control plane running in the seed, or to/from the shoot's system components running on the worker nodes.
* Change the configuration of an existing control plane/system component.
* Restart of existing control plane/system components (this might result in a short unavailability of the Kubernetes API server, e.g., when etcd or a kube-apiserver itself is being restarted)

### Behavioural Changes

Generally, some of such updates (e.g., configuration changes) could theoretically result in different behaviour of controllers.
If such changes would be backwards-incompatible, then we usually follow one of those approaches (depends on the concrete change):

* Only apply the change for new clusters.
* Expose a new field in the `Shoot` resource that lets users control this changed behaviour to enable it at a convenient point in time.
* Put the change behind an alpha feature gate (disabled by default) in the gardenlet (only controllable by Gardener operators), which will be promoted to beta (enabled by default) in subsequent releases (in this case, end-users have no influence on when the behaviour changes - Gardener operators should inform their end-users and provide clear timelines when they will enable the feature gate).

## Upgrades

We consider shoot upgrades to change either the:

* Kubernetes version (`.spec.kubernetes.version`)
* Kubernetes version of the worker pool if specified (`.spec.provider.workers[].kubernetes.version`)
* Machine image version of at least one worker pool (`.spec.provider.workers[].machine.image.version`)

Generally, an upgrade is also performed through a reconciliation of the `Shoot` resource, i.e., the same concepts as for [shoot updates](#updates) apply.
If an end-user triggers an upgrade (e.g., by changing the Kubernetes version) after a new Gardener version was deployed but before the shoot was reconciled again, then this upgrade might incorporate the changes delivered with this new Gardener version.

The `UpdateStrategy` field in the Shoot specification (`.spec.provider.workers[].updateStrategy`) gives users the flexibility to define how the `machine-controller-manager` handles worker pool updates during the upgrade process. Currently gardener support three update strategies:
* `AutoRollingUpdate`
* `AutoInPlaceUpdate`
* `ManualInPlaceUpdate`

> ⚠️ The above strategies generally require draining the node when changes are made to the `Shoot` specification. The specific changes that trigger this behavior will be discussed in later sections.
For all other spec changes like Kubernetes patch version update, the upgrade is executed without draining the node and the shoot worker nodes remain unchanged. In case of Kubernetes patch version, only the `kubelet` process is restarted with the updated Kubernetes version binary.

### Rolling Updates

The upgrade is performed in a "rolling update" manner, during which nodes in the worker pool are replaced. Similar to how pods in Kubernetes are updated when backed by a `Deployment`.
Worker nodes are terminated one after another and replaced by new nodes.
The existing workload is gracefully drained and evicted from the old worker nodes to new worker nodes, respecting the configured `PodDisruptionBudget`s (see [Specifying a Disruption Budget for your Application](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)).

#### Automatic Rolling Updates

When the auto rolling update strategy is selected, the update process is fully orchestrated by the Gardener and the `machine-controller-manager`. The `machine-controller-manager` sequentially terminates the worker nodes and replaces them with new nodes.
>ℹ️  This is the `default` update strategy.

To create workers with `AutoRollingUpdate`, either omit the `Shoot'`s `.spec.provider.workers[].updateStrategy` field (it will default to `AutoRollingUpdate`) or explicitly set the field to `AutoRollingUpdate`.

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      maxSurge: 0
      maxUnavailable: 2
      updateStrategy: AutoRollingUpdate
```

#### Customize Rolling Update Behaviour of Shoot Worker Nodes

The `.spec.provider.workers[]` list exposes two fields that you might configure based on your workload's needs: `maxSurge` and `maxUnavailable`.
The same concepts [like in Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment) apply.
Additionally, you might customize how the `machine-controller-manager` is behaving. You can configure the following fields in `.spec.provider.worker[].machineControllerManager`:

* `machineDrainTimeout`: Timeout (in duration) used while draining of machine before deletion, beyond which `machine-controller-manager` forcefully deletes the machine (default: `2h`).
* `machineHealthTimeout`: Timeout (in duration) used while re-joining (in case of temporary health issues) of a machine before it is declared as failed (default: `10m`).
* `machineCreationTimeout`: Timeout (in duration) used while joining (during creation) of a machine before it is declared as failed (default: `10m`).
* `maxEvictRetries`: Maximum number of times evicts would be attempted on a pod before it is forcibly deleted during the draining of a machine (default: `10`).
* `nodeConditions`: List of case-sensitive node-conditions which will change a machine to a `Failed` state after the `machineHealthTimeout` duration. It may further be replaced with a new machine if the machine is backed by a machine-set object (defaults: `KernelDeadlock`, `ReadonlyFilesystem` , `DiskPressure`).

#### Rolling Update Triggers

A rolling update of the shoot worker nodes is triggered for some changes to your worker pool specification (`.spec.provider.workers[]`, even if you don't change the Kubernetes or machine image version).
The complete list of fields that trigger a rolling update:

* `.spec.kubernetes.version` (except for patch version changes)
* `.spec.provider.workers[].machine.image.name`
* `.spec.provider.workers[].machine.image.version`
* `.spec.provider.workers[].machine.type`
* `.spec.provider.workers[].volume.type`
* `.spec.provider.workers[].volume.size`
* `.spec.provider.workers[].providerConfig` (provider extension dependent with feature gate `NewWorkerPoolHash`)
* `.spec.provider.workers[].cri.name`
* `.spec.provider.workers[].kubernetes.version` (except for patch version changes)
* `.spec.systemComponents.nodeLocalDNS.enabled`
* `.status.credentials.rotation.certificateAuthorities.lastInitiationTime` (changed by Gardener when a shoot CA rotation is initiated) when worker pool is not part of `.status.credentials.rotation.certificateAuthorities.pendingWorkersRollouts[]`
* `.status.credentials.rotation.serviceAccountKey.lastInitiationTime` (changed by Gardener when a shoot service account signing key rotation is initiated) when worker pool is not part of `.status.credentials.rotation.serviceAccountKey.pendingWorkersRollouts[]`

If feature gate `NewWorkerPoolHash` is enabled:

* `.spec.kubernetes.kubelet.kubeReserved` (unless a worker pool-specific value is set)
* `.spec.kubernetes.kubelet.systemReserved` (unless a worker pool-specific value is set)
* `.spec.kubernetes.kubelet.evictionHard` (unless a worker pool-specific value is set)
* `.spec.kubernetes.kubelet.cpuManagerPolicy` (unless a worker pool-specific value is set)
* `.spec.provider.workers[].kubernetes.kubelet.kubeReserved`
* `.spec.provider.workers[].kubernetes.kubelet.systemReserved`
* `.spec.provider.workers[].kubernetes.kubelet.evictionHard`
* `.spec.provider.workers[].kubernetes.kubelet.cpuManagerPolicy`

Changes to `kubeReserved` or `systemReserved` do not trigger a node roll if their sum does not change.

Generally, the provider extension controllers might have additional constraints for changes leading to rolling updates, so please consult the respective documentation as well.
In particular, if the feature gate `NewWorkerPoolHash` is enabled and a worker pool uses the new hash, then the `providerConfig` as a whole is not included. Instead only fields selected by the provider extension are considered.

### In-Place Updates

For scenarios where users want to retain the current nodes and avoid deletion during updates, Gardener provides the option of `in-place` updates. The upgrade is performed without replacing the underlying machines. Although there is an exception where new nodes are created with the updated configuration and old ones are terminated. One such exception is discussed in [Automatic inplace update](#automatic-in-place-updates) section.

The existing workload is gracefully drained and evicted from the worker nodes, respecting the configured `PodDisruptionBudget`s (see [Specifying a Disruption Budget for your Application](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)).

> ℹ️ Currently, `in-place` updates are controlled by the `InPlaceNodeUpdates` feature gate in the `gardener-apiserver`.

For in-place updates, the first requirement is that the operating system must support them. For a specific machine image version, the configuration for in-place updates must be defined in the `CloudProfile` under `spec.machineImages[].versions[].inPlaceUpdates`:
- The `inPlaceUpdates.supported` field must be set to `true`.
- The `inPlaceUpdates.minVersionForUpdate` field specifies the minimum version from which an in-place update to the target machine image version can be performed.

```yaml
machineImages:
- name: gardenlinux
  versions:
  - version: 1632.0.0
    inPlaceUpdates:
      supported: true
      minVersionForUpdate: 1630.0.0
```

The `inPlaceUpdates` field in the Shoot status provides details about in-place updates for the Shoot workers. It includes the `pendingWorkerUpdates` field, which lists the worker pools that are awaiting in-place updates.

#### Customize In-Place Update Behaviour of Shoot Worker Nodes

In addition to customisable fields mentioned in [](#customize-rolling-update-behaviour-of-shoot-worker-nodes) section, you can configure the following fields in `.spec.provider.worker[].machineControllerManager`:

* `MachineInPlaceUpdateTimeout`:  Timeout (in duration) after which an in-place update is declared as failed.
* `DisableHealthTimeout`: A boolean value that, when set to `true`, ignores the health timeout. As a result, machines are never marked as failed, and unhealthy machines are not deleted. The default value is `true` for in-place updates.

#### In-Place Update Triggers

An in-place update of the shoot worker nodes is triggered for rolling update triggers listed under [Rolling Update Triggers](#rolling-update-triggers) except for the following:
* `.spec.provider.workers[].machine.image.name`
* `.spec.provider.workers[].machine.type`
* `.spec.provider.workers[].volume.type`
* `.spec.provider.workers[].volume.size`
* `.spec.provider.workers[].cri.name`
* `.spec.systemComponents.nodeLocalDNS.enabled`

> There are validations which restricts changing the above mentioned exception fields when `in-place` updates strategy is configured.

When a worker pool is undergoing an in-place update, applying subsequent updates to the same worker pool is restricted.
If an in-place update fails and nodes are left in a problematic state, user intervention is required to manually fix the nodes. In cases where a subsequent update is necessary to resolve the issue, users can update the worker pool after adding the force update annotation `gardener.cloud/operation=force-in-place-update` on the Shoot. Refer to [Force-update a worker pool with InPlace update strategy](shoot_operations.md#force-update-a-worker-pool-with-inplace-update-strategy) for more details.

> ⚠️ Changing the update strategy from `AutoRollingUpdate` to `AutoInPlaceUpdate`/`ManualInPlaceUpdate` (and vice versa) is not allowed. However, switching between `AutoInPlaceUpdate` and `ManualInPlaceUpdate` is permitted.

#### Automatic In-Place Updates

In case of AutoInPlaceUpdate update strategy, the update process is fully orchestrated by Gardener and the `machine-controller-manager`. No user intervention is required.
Set `.spec.provider.workers[].updateStrategy` field in the `Shoot` spec to `AutoInPlaceUpdate`.

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      maxSurge: 0
      maxUnavailable: 2
      updateStrategy: AutoInPlaceUpdate
```

During automatic in-place updates, if the `maxSurge` value is set to greater than 0, the `machine-controller-manager` creates new nodes equal to the `maxSurge` value. All old nodes, except for those equal to the `maxSurge` value, are updated in place, and the old nodes corresponding to the `maxSurge` value are terminated. If `maxSurge` is set to `0`, no new nodes are created and all old nodes are updated in-place.

The `inPlaceUpdates.pendingWorkerUpdates.autoInPlaceUpdate` field in the Shoot status lists the names of worker pools that are pending updates with this strategy.

#### Manual In-Place Updates

The `ManualInPlaceUpdate` strategy allows users to control and orchestrate the update process manually.
Set `.spec.provider.workers[].updateStrategy` field in the `Shoot` spec to `ManualInPlaceUpdate`.

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      maxSurge: 0
      maxUnavailable: 2
      updateStrategy: ManualInPlaceUpdate
```

Once `machine-controller-manager` labels nodes with `node.machine.sapcloud.io/candidate-for-update`, user can select the candidate nodes for update by labeling them with `node.machine.sapcloud.io/selected-for-update=true`:

```sh
kubectl label node <node-name> node.machine.sapcloud.io/selected-for-update=true
```

The `ManualInPlaceWorkersUpdated` [constraint](../shoot/shoot_status.md#constraints) in the shoot status indicates that at least one worker pool with the `ManualInPlaceUpdate` strategy is pending an update. Shoot reconciliation will still succeed even if there are worker pools pending updates.

The `inPlaceUpdates.pendingWorkerUpdates.manualInPlaceUpdate` field in the `Shoot` status lists the names of worker pools that are pending updates with this strategy.

## Related Documentation

* [Shoot Operations](shoot_operations.md)
* [Shoot Maintenance](../shoot/shoot_maintenance.md)
* [Confine Specification Changes/Updates Roll Out To Maintenance Time Window](../shoot/shoot_maintenance.md#confine-specification-changesupdates-roll-out).
