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

`UpdateStrategy` field under shoot worker enables the end user to choose between different behaviour of MCM during the upgrade process. Currently gardener support three update strategies:
* `AutoRollingUpdate`
* `AutoInPlaceUpdate`
* `ManualInPlaceUpdate`



### In-Place vs. Rolling Updates

In case of in-place update worker nodes are updated without the need for replacing the VMs with the new ones. This means that the shoot worker nodes remain untouched. There is some downtime expected as the node needs to be drained and then workload will be scheduled again once the node is updated. If the Kubernetes patch version is changed, then the upgrade is always in-place.

In case of rolling update, upgrade is done in a "rolling update" fashion, similar to how pods in Kubernetes are updated (when backed by a `Deployment`).
The worker nodes will be terminated one after another and replaced by new machines.
The existing workload is gracefully drained and evicted from the old worker nodes to new worker nodes, respecting the configured `PodDisruptionBudget`s (see [Specifying a Disruption Budget for your Application](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)).

### AutoRollingUpdate

When auto rollling update strategy is selected, upgrade is done in a "rolling update" fashion. Machine controller manager terminates the worker nodes one after the other and replaces it with new machines. This is the default update strategy. In order to create workers with `AutoRollingUpdate` either skip setting the `workers.updateStrategy` field (in this case the field will be defualted to `AutoInPlaceUpdate`) or set the field explicitly to `AutoInPlaceUpdate` for each workers.

```yaml
spec:
  provider:
    workers:
      - name: cpu-worker
        minimum: 5
        maximum: 5
        maxSurge: 0
        maxUnavailable: 2
        updateStrategy: AutoRollingUpdate
        machine:
          type: m5.large
          image:
            name: <some-image-name>
            version: <some-image-version>
          architecture: <some-cpu-architecture>
        providerConfig: <some-machine-image-specific-configuration>
```
    
Note: Infra need to have enough resource to create new nodes.

#### Customize Rolling Update Behaviour of Shoot Worker Nodes

The `.spec.provider.workers[]` list exposes two fields that you might configure based on your workload's needs: `maxSurge` and `maxUnavailable`.
The same concepts [like in Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment) apply.
Additionally, you might customize how the machine-controller-manager (abbrev.: MCM; the component instrumenting this rolling update) is behaving. You can configure the following fields in `.spec.provider.worker[].machineControllerManager`:

* `machineDrainTimeout`: Timeout (in duration) used while draining of machine before deletion, beyond which MCM forcefully deletes the machine (default: `2h`).
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
* `.spec.provider.workers[].providerConfig` (except if feature gate `NewWorkerPoolHash`)
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

### InPlace updates

For the scenarios where user would like to keep the current node and avoid deletion of such nodes even after the updates, Gardener comes with the option of `in-place` update. Currently `in-place` update is kept under `InPlaceNodeUpdates` feature gate.

Machine image version in the cloudprofile has the `inPlaceUpdates` field which contains the configuration for in-place updates. For an OS to support in-place update `inPlaceUpdates.supported` field should be set to true. Moreoever `inPlaceUpdates.minVersionForInPlaceUpdate` specifies the minimum supported version from which an in-place update to this machine image version can be performed.

```yaml
machineImages:
- name: gardenlinux
  updateStrategy: minor
  versions:
  - architectures:
    - amd64
    - arm64
    classification: preview
    cri:
    - containerRuntimes:
      - type: gvisor
      name: containerd
    version: 1630.0.0-inplace-update-poc
    inPlaceUpdates:
      supported: true
```

#### InPlace Update Triggers

An in-place update of the shoot worker nodes is triggered for rolling update triggers listed under [Rolling Update Triggers](#rolling-update-triggers) except the following:
* `.spec.provider.workers[].machine.image.name`
* `.spec.provider.workers[].machine.type`
* `.spec.provider.workers[].volume.type`
* `.spec.provider.workers[].volume.size`
* `.spec.provider.workers[].cri.name`
* `.spec.systemComponents.nodeLocalDNS.enabled`

#### Validations for In-Place update

Validations are in place to restrict any further updates when the current update is going on.
For the cases where in-place update fails and the nodes are in problematic state, it requires user intervention to fix the node manually . After the fix user should be able to restart the update by the force update annotation `gardener.cloud/operation=force-in-place-update`. Without this annotation, any subsequent updates to the same worker pool are denied, refer [Force-update a worker pool with InPlace update strategy](shoot_operations.md#force-update-a-worker-pool-with-inplace-update-strategy) on how to force an update for worker pools.

At present it is not allowed to change the update strategy of a worker from AutoRollingUpdate to Auto/Manual in-place update. Same applies for the other way round as well that is once the update strategy is selected as Auto/Manual in-place update , it cannot be changed to AutoRolling update. Though switching between Auto and Manual auto in-place update is allowed. 

#### AutoInPlaceUpdate

In case of AutoInPlaceUpdate update strategy, the update process is fully carried out by the MCM and gardener without user intervention. Use `updateStrategy` as `AutoInPlaceUpdate` in worker for auto in-place update.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: crazy-botany
  namespace: garden-dev
spec:
  secretBindingName: my-provider-account
  cloudProfile:
    name: cloudprofile1
  region: europe-central-1
  provider:
    type: <some-provider-name> # {aws,azure,gcp,...}
    workers:
      - name: cpu-worker
        minimum: 5
        maximum: 5
        maxSurge: 0
        maxUnavailable: 2
        updateStrategy: AutoInPlaceUpdate
        machine:
          type: m5.large
          image:
            name: <some-image-name>
            version: <some-image-version>
          architecture: <some-cpu-architecture>
        providerConfig: <some-machine-image-specific-configuration>
  kubernetes:
    version: 1.27.3
  networking:
```

#### Customize Auto In-Place Update Behaviour of Shoot Worker Nodes

The `.spec.provider.workers[]` list exposes two fields that you might configure based on your workload's needs: `maxSurge` and `maxUnavailable`.
The same concepts [like in Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment) apply.
Additionally, you might customize how the machine-controller-manager (abbrev.: MCM; the component instrumenting this rolling update) is behaving. You can configure the following fields in `.spec.provider.worker[].machineControllerManager`:

* `MachineInPlaceUpdateTimeout`:  Timeout (in duration) after which an in-place update is declared as failed.
* `DisableHealthTimeout`: Boolean value if set to true, health timeout will be ignored, leading to machine never being declared as failed (default: `true` for in-place updates). 

#### ManualInPlaceUpdate

ManualInPlaceUpdate update strategy is useful in scenarios where user wants to control and drive the update process. Here user has the option to select the particular node which needs to be updated at a time. In order to update a node, label the node with `node.machine.sapcloud.io/selected-for-update=true`. Once the node is labeled  with `node.machine.sapcloud.io/selected-for-update=true`, MCM will carry out the update process.

`ManualInPlaceWorkersUpdated` [constraint](../shoot/shoot_status.md/#constraints) in shoot status indicates that at least one worker pool with the update strategy `ManualInPlaceUpdate` is pending an update. Shoot reconciliation succeeds even with worker pools pending for an update.

<!-- mention about manual inplace credential rotation in the credential rotation doc-->

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: crazy-botany
  namespace: garden-dev
spec:
  secretBindingName: my-provider-account
  cloudProfile:
    name: cloudprofile1
  region: europe-central-1
  provider:
    type: <some-provider-name> # {aws,azure,gcp,...}
    workers:
      - name: cpu-worker
        minimum: 5
        maximum: 5
        maxSurge: 0
        maxUnavailable: 2
        updateStrategy: ManualInPlaceUpdate
        machine:
          type: m5.large
          image:
            name: <some-image-name>
            version: <some-image-version>
          architecture: <some-cpu-architecture>
        providerConfig: <some-machine-image-specific-configuration>
  kubernetes:
    version: 1.27.3
  networking:
```

#### Customize Manual InPlace Update Behaviour of Shoot Worker Nodes

The `.spec.provider.workers[]` list exposes two fields that you might configure based on your workload's needs: `maxSurge` and `maxUnavailable`. In case of manual in-place update, `maxSurge` is defaulted to `0` and `maxUnavailable` is defaulted to `1`. `maxSurge` value is always taken as `0` for manual in-place update irrespective of value set by the end user.

Additionally, `.spec.provider.worker[].machineControllerManager` can be customised in same way as mentioned in [Customize Auto InPlace Update Behaviour of Shoot Worker Nodes](#customize-auto-inplace-update-behaviour-of-shoot-worker-nodes).

## Related Documentation

* [Shoot Operations](shoot_operations.md)
* [Shoot Maintenance](../shoot/shoot_maintenance.md)
* [Confine Specification Changes/Updates Roll Out To Maintenance Time Window](../shoot/shoot_maintenance.md#confine-specification-changesupdates-roll-out).
