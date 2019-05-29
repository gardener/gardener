# General conventions

All the extensions that are registered to Gardener are deployed to the seed clusters (at the moment, every extension is installed to every seed cluster, however, in the future Gardener will be more smart to determine which extensions needs to be placed into which seed).

Some of these extensions might need to create global resources in the seed (e.g., `ClusterRole`s), i.e., it's important to have a naming scheme to avoid conflicts as it cannot be checked or validated upfront that two extensions don't use the same names.

Consequently, this page should help answering some general questions that might come up when it comes to developing an extension.

## Is there a naming scheme for (global) resources?

As there is no formal process to validate non-existence of conflicts between two extensions please follow these naming schemes when creating resources (especially, when creating global resources, but it's in general a good idea for most created resources):

*The resource name should be prefixed with `extensions.gardener.cloud:<extension-type>-<extension-name>:<resource-name>`*, for example:

* `extensions.gardener.cloud:provider-aws:machine-controller-manager`
* `extensions.gardener.cloud:extension-certificate-service:cert-broker`

## How to create resources in the shoot cluster?

Some extensions might not only create resources in the seed cluster itself but also in the shoot cluster. Usually, every extension comes with a `ServiceAccount` and the required RBAC permissions when it gets installed to the seed.
However, there are no credentials for the shoot for every extension.

Gardener creates a kubeconfig for itself that it uses to interact with the shoot cluster.
This kubeconfig is stored as a `Secret` with name [`gardener`](https://github.com/gardener/gardener/blob/master/pkg/apis/core/v1alpha1/types_constants.go) in the shoot namespace.
Extension controllers may use this kubeconfig to interact with the shoot cluster if desired (it has full administrator privileges and no further RBAC rules are required).
Instead, they could also create their own kubeconfig for every shoot (which, of course, is better for auditing reasons, but not yet enforced at this point in time).

## How to create certificates/kubeconfigs for the shoot cluster?

Gardener creates several certificate authorities (CA) that are used to create server/client certificates for various components.
For example, the shoot's etcd has its own CA, the kube-aggregator has its own CA as well, and both are different to the actual cluster's CA.

These CAs are stored as `Secret`s in the shoot namespace (see [this](https://github.com/gardener/gardener/blob/master/pkg/apis/core/v1alpha1/types_constants.go) for the actual names).
Extension controllers may use them to create further certificates/kubeconfigs for potential other components they need to deploy to the seed or shoot.
[These utility functions](https://github.com/gardener/gardener/tree/master/pkg/utils/secrets) should help with the creation and management.
