# Deploy resources to the Shoot cluster

We have introduced a component called [`gardener-resource-manager`](https://github.com/gardener/gardener-resource-manager) that is deployed as part of every shoot control plane in the seed.
This component manages CRDs, so called `ManagedResource`s.
Managed resources contain Kubernetes resources that shall be created, reconciled, updated, and deleted by the gardener-resource-manager.

Extension controllers may create these `ManagedResource`s in the shoot namespace if they need to create any resource in the shoot cluster itself, for example RBAC roles (or anything else).

## Where can I find more examples and more information how to use `ManagedResource`s?

Please take a look at the [README.md](https://github.com/gardener/gardener-resource-manager/blob/master/README.md) in the [gardener/gardener-resource-manager](https://github.com/gardener/gardener-resource-manager) repository.
