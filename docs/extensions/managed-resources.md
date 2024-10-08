# Deploy Resources to the Shoot Cluster

We have introduced a component called [`gardener-resource-manager`](../concepts/resource-manager.md) that is deployed as part of every shoot control plane in the seed.
One of its tasks is to manage CRDs, so called `ManagedResource`s.
Managed resources contain Kubernetes resources that shall be created, reconciled, updated, and deleted by the gardener-resource-manager.

Extension controllers may create these `ManagedResource`s in the shoot namespace if they need to create any resource in the shoot cluster itself, for example RBAC roles (or anything else).

## Where can I find more examples and more information how to use `ManagedResource`s?

Please take a look at the [respective documentation](../concepts/resource-manager.md).
