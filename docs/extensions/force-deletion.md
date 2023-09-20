# Force Deletion

From `v1.80`, Gardener supports [Shoot Force Deletion](../usage/shoot_operations.md#force-deletion). All extension controllers should also properly support it. This document outlines some important points that extension maintainers should keep in mind to support force deletion in their extensions.

## Overall Principles

The following principles should always be upheld:

* All resources pertaining to the extension and managed by it should be appropriately handled and cleaned up by the extension when force deletion is initiated.

## Implementation Details

### ForceDelete Actuator Methods

Most extension controller implementations follow a common pattern where a generic `Reconciler` implementation delegates to an `Actuator` interface that contains the methods `Reconcile`, `Delete`, `Migrate` and `Restore` provided by the extension. A new method, `ForceDelete` has been added to all such `Actuator` interfaces; see [the infrastructure `Actuator` interface](https://github.com/gardener/gardener/blob/master/extensions/pkg/controller/infrastructure/actuator.go) as an example. The generic reconcilers call this method if the Shoot has annotation `confirmation.gardener.cloud/force-deletion=true`. Thus, it should be implemented by the extension controller to forcefully delete resources if not possible to delete them gracefully. If graceful deletion is possible, then in the `ForceDelete`, they can simply call the `Delete` method.

### Extension Controllers Based on Generic Actuators

In practice, the implementation of many extension controllers (for example, the controlplane and worker controllers in most provider extensions) are based on a *generic `Actuator` implementation* that only delegates to extension methods for behavior that is truly provider-specific. In all such cases, the `ForceDelete` method has already been implemented with a method that should suit most of the extensions. If it doesn't suit your extension, then the `ForceDelete` method needs to be overridden; see the [Azure controlplane controller](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/controlplane) as an example.

### Extension Controllers Not Based on Generic Actuators

The implementation of some extension controllers (for example, the infrastructure controllers in all provider extensions) are not based on a generic `Actuator` implementation. Such extension controllers must always provide a proper implementation of the `ForceDelete` method according to the above guidelines; see the [AWS infrastructure controller](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller/infrastructure) as an example. In practice, this might result in code duplication between the different extensions, since the `ForceDelete` code is usually not OS-specific.