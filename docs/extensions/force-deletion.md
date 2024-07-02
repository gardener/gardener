# Force Deletion

From `v1.81`, Gardener supports [Shoot Force Deletion](../usage/operating_through_annotations/shoot_operations.md#force-deletion). All extension controllers should also properly support it. This document outlines some important points that extension maintainers should keep in mind to support force deletion in their extensions.

## Overall Principles

The following principles should always be upheld:

* All resources pertaining to the extension and managed by it should be appropriately handled and cleaned up by the extension when force deletion is initiated.

## Implementation Details

### ForceDelete Actuator Methods

Most extension controller implementations follow a common pattern where a generic `Reconciler` implementation delegates to an `Actuator` interface that contains the methods `Reconcile`, `Delete`, `Migrate` and `Restore` provided by the extension. A new method, `ForceDelete` has been added to all such `Actuator` interfaces; see [the infrastructure `Actuator` interface](../../extensions/pkg/controller/infrastructure/actuator.go) as an example. The generic reconcilers call this method if the Shoot has annotation `confirmation.gardener.cloud/force-deletion=true`. Thus, it should be implemented by the extension controller to forcefully delete resources if not possible to delete them gracefully. If graceful deletion is possible, then in the `ForceDelete`, they can simply call the `Delete` method.

### Extension Controllers Based on Generic Actuators

In practice, the implementation of many extension controllers (for example, the controlplane and worker controllers in most provider extensions) are based on a *generic `Actuator` implementation* that only delegates to extension methods for behavior that is truly provider-specific. In all such cases, the `ForceDelete` method has already been implemented with a method that should suit most of the extensions. If it doesn't suit your extension, then the `ForceDelete` method needs to be overridden; see the [Azure controlplane controller](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/controlplane) as an example.

### Extension Controllers Not Based on Generic Actuators

The implementation of some extension controllers (for example, the infrastructure controllers in all provider extensions) are not based on a generic `Actuator` implementation. Such extension controllers must always provide a proper implementation of the `ForceDelete` method according to the above guidelines; see the [AWS infrastructure controller](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller/infrastructure) as an example. In practice, this might result in code duplication between the different extensions, since the `ForceDelete` code is usually not OS-specific.

### Some General Implementation Examples
- If the extension deploys only resources in the shoot cluster not backed by infrastructure in third-party systems, then performing the regular deletion code (`actuator.Delete`) will suffice in the majority of cases. (e.g - https://github.com/gardener/gardener-extension-shoot-networking-filter/blob/1d95a483d803874e8aa3b1de89431e221a7d574e/pkg/controller/lifecycle/actuator.go#L175-L178)
- If the extension deploys resources which are backed by infrastructure in third-party systems:
  - If the resource is in the Seed cluster, the extension should remove the finalizers and delete the resource. This is needed especially if the resource is a custom resource since `gardenlet` will not be aware of this resource and cannot take action.
  - If the resource is in the Shoot and if it's deployed by a `ManagedResource`, then `gardenlet` will take care to forcefully delete it in a later step of force-deletion. If the resource is not deployed via a `ManagedResource`, then it wouldn't block the deletion flow anyway since it is in the Shoot cluster. In both cases, the extension controller can ignore the resource and return `nil`.
