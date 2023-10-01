# Control Plane Migration

*Control Plane Migration* is a new Gardener feature that has been recently implemented as proposed in [GEP-7 Shoot Control Plane Migration](../proposals/07-shoot-control-plane-migration.md). It should be properly supported by all extensions controllers. This document outlines some important points that extension maintainers should keep in mind to properly support migration in their extensions.

## Overall Principles

The following principles should always be upheld:

* All states maintained by the extension that is external from the seed cluster, for example infrastructure resources in a cloud provider, DNS entries, etc., should be kept during the migration. No such state should be deleted and then recreated, as this might cause disruption in the availability of the shoot cluster.
* All Kubernetes resources maintained by the extension in the shoot cluster itself should also be kept during the migration. No such resources should be deleted and then recreated.

## Migrate and Restore Operations

Two new operations have been introduced in Gardener. They can be specified as values of the `gardener.cloud/operation` annotation on an extension resource to indicate that an operation different from a normal `reconcile` should be performed by the corresponding extension controller:

* The `migrate` operation is used to ask the extension controller in the source seed to stop reconciling extension resources (in case they are requeued due to errors) and perform cleanup activities, if such are required. These cleanup activities might involve removing finalizers on resources in the shoot namespace that have been previously created by the extension controller and deleting them without actually deleting any resources external to the seed cluster. This is also the last opportunity for extensions to persist their state into the `.status.state` field of the reconciled extension resource before its restored in the new destination seed cluster.
* The `restore` operation is used to ask the extension controller in the destination seed to restore any state saved in the extension resource `status`, before performing the actual reconciliation.

Unlike the [reconcile operation](https://github.com/gardener/gardener/blob/master/docs/extensions/reconcile-trigger.md), extension controllers must remove the `gardener.cloud/operation` annotation at the end of a successful reconciliation when the current operation is `migrate` or `restore`, not at the beginning of a reconciliation.

## Cleaning-Up Source Seed Resources

All resources in the source seed that have been created by an extension controller, for example secrets, config maps, [managed resources](managedresources.md), etc., should be properly cleaned up by the extension controller when the current operation is `migrate`. As mentioned above, such resources should be deleted without actually deleting any resources external to the seed cluster.

There is one exception to this: `Secret`s labeled with `persist=true` created via the [secrets manager](../development/secrets_management.md). They should be kept (i.e., the `Cleanup` function of secrets manager should not be called) and will be garbage collected automatically at the end of the `migrate` operation. This ensures that they can be properly persisted in the `ShootState` resource and get restored on the new destination seed cluster.

For many custom resources, for example MCM resources, the above requirement means in practice that any finalizers should be removed before deleting the resource, in addition to ensuring that the resource deletion is not reconciled by its respective controller if there is no finalizer. For managed resources, the above requirement means in practice that the `spec.keepObjects` field should be set to `true` before deleting the extension resource.

Here it is assumed that any resources that contain state needed by the extension controller can be safely deleted, since any such state has been saved as described in [Saving and Restoring Extension States](#saving-and-restoring-extension-states) at the end of the last successful reconciliation.

## Saving and Restoring Extension States

Some extension controllers create and maintain their own state when reconciling extension resources. For example, most infrastructure controllers use Terraform and maintain the terraform state in a special config map in the shoot namespace. This state must be properly migrated to the new seed cluster during control plane migration, so that subsequent reconciliations in the new seed could find and use it appropriately.

All extension controllers that require such state migration must save their state in the `status.state` field of their extension resource at the end of a successful reconciliation. They must also restore their state from that same field upon reconciling an extension resource when the current operation is `restore`, as specified by the `gardener.cloud/operation` annotation, before performing the actual reconciliation.

As an example, an infrastructure controller that uses Terraform must save the terraform state in the `status.state` field of the `Infrastructure` resource. An `Infrastructure` resource with a properly saved state might look as follows:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Infrastructure
metadata:
  name: infrastructure
  namespace: shoot--foo--bar
spec:
  type: azure
  region: eu-west-1
  secretRef:
    name: cloudprovider
    namespace: shoot--foo--bar
  providerConfig:
    apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
    kind: InfrastructureConfig
    resourceGroup:
      name: mygroup
    ...
status:
  state: |
    {
      "version": 3,
      "terraform_version": "0.11.14",
      "serial": 2,
      "lineage": "3a1e2faa-e7b6-f5f0-5043-368dd8ea6c10",
      ...
    }
```

Extension controllers that do not use a saved state and therefore do not require state migration could leave the `status.state` field as `nil` at the end of a successful reconciliation, and just perform a normal reconciliation when the current operation is `restore`.

In addition, extension controllers that use [referenced resources](referenced-resources.md) (usually secrets) must also make sure that these resources are added to the `status.resources` field of their extension resource at the end of a successful reconciliation, so they could be properly migrated by Gardener to the destination seed.

## Implementation Details

### Migrate and Restore Actuator Methods

Most extension controller implementations follow a common pattern where a generic `Reconciler` implementation delegates to an `Actuator` interface that contains the methods `Reconcile` and `Delete`, provided by the extension.
Two methods `Migrate` and `Restore` are available in all such `Actuator` interfaces, see [the infrastructure `Actuator` interface](https://github.com/gardener/gardener/blob/master/extensions/pkg/controller/infrastructure/actuator.go) as an example.
These methods are called by the generic reconcilers for the [migrate and restore operations](#migrate-and-restore-operations) respectively, and should be implemented by the extension according to the above guidelines.

### Extension Controllers Based on Generic Actuators

In practice, the implementation of many extension controllers (for example, the `ControlPlane` and `Worker` controllers in most provider extensions) are based on a *generic `Actuator` implementation* that only delegates to extension methods for behavior that is truly provider specific.
In all such cases, the `Migrate` and `Restore` methods have already been implemented properly in the generic actuators and there is nothing more to do in the extension itself.

In some rare cases, extension controllers based on a generic actuator might still introduce a custom `Actuator` implementation to override some of the generic actuator methods in order to enhance or change their behavior in a certain way.
In such cases, the `Migrate` and `Restore` methods might need to be overridden as well, see the [Azure controlplane controller](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/controlplane) as an example.

#### `Worker` State

Note that the machine state is handled specially by `gardenlet` (i.e., all relevant objects in the `machine.sapcloud.io/v1alpha1` API are directly persisted by `gardenlet` and **NOT** by the generic actuators).
In the past, they were persisted to the `Worker`'s `.status.state` field by the so-called "worker state reconciler", however, this reconciler was dropped and changed as part of [GEP-22](../proposals/22-improved-usage-of-shootstate-api.md#eliminating-the-worker-state-reconciler).
Nowadays, `gardenlet` directly writes the state to the `ShootState` resource during the `Migrate` phase of a `Shoot` (without the detour of the `Worker`'s `.status.state` field).
On restoration, unlike for other extension kinds, `gardenlet` no longer populates the machine state into the `Worker`'s `.status.state` field.
Instead, the extension controller should read the machine state directly from the `ShootState` in the garden cluster (see [this document](garden-api-access.md) for information how to access the garden cluster) and use it to subsequently restore the relevant `machine.sapcloud.io/v1alpha1` resources.
This flow is implemented in the [generic `Worker` actuator](../../extensions/pkg/controller/worker/genericactuator/actuator_restore.go).
As a result, Extension controllers using this generic actuator do not need to implement any custom logic.

### Extension Controllers Not Based on Generic Actuators

The implementation of some extension controllers (for example, the infrastructure controllers in all provider extensions) are not based on a generic `Actuator` implementation.
Such extension controllers must always provide a proper implementation of the `Migrate` and `Restore` methods according to the above guidelines, see the [AWS infrastructure controller](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller/infrastructure) as an example.
In practice, this might result in code duplication between the different extensions, since the `Migrate` and `Restore` code is usually not provider or OS-specific.

> If you do not use the generic `Worker` actuator, see [this section](#worker-state) for information how to handle the machine state related to the `Worker` resource.
