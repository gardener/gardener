# General conventions

All the extensions that are registered to Gardener are deployed to the seed clusters, on which they are required (also see [ControllerRegistration](controllerregistration.md)).

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

Extensions are supposed to use [`ManagedResources`](../concepts/resource-manager.md#ManagedResource-controller) to manage resources in shoot clusters.
gardenlet deploys gardener-resource-manager instances into all shoot control planes, that will reconcile `ManagedResources` without a specified class (`spec.class=null`) in shoot clusters.

If you need to deploy a non-DaemonSet resource you need to ensure that it only runs on nodes that are allowed to host system components and extensions.
To do that you need to configure a `nodeSelector` as following:
 ```yaml
nodeSelector:
  worker.gardener.cloud/system-components: "true"
```

## How to create kubeconfigs for the shoot cluster?

Historically, Gardener extensions used to generate kubeconfigs with client certificates for components they deploy into the shoot control plane.
For this, they reused the shoot cluster CA secret (`ca`) to issue new client certificates.
With [gardener/gardener#4661](https://github.com/gardener/gardener/issues/4661) we moved away from using client certificates in favor of short-lived auto-rotated `ServiceAccount` tokens. These tokens are managed by gardener-resource-manager's [`TokenRequestor`](../concepts/resource-manager.md#tokenrequestor).
Extensions are supposed to reuse this mechanism for requesting tokens and a `generic-token-kubeconfig` for authenticating against shoot clusters.

With [GEP-18](../proposals/18-shoot-CA-rotation.md) (Shoot cluster CA rotation), a dedicated CA will be used for signing client certificates ([gardener/gardener#5779](https://github.com/gardener/gardener/pull/5779)), that will be rotated when triggered by the shoot owner.
With this, extensions cannot reuse the `ca` secret anymore to issue client certificates.
Hence, extensions must switch to short-lived `ServiceAccount` tokens in order to support the CA rotation feature.

The `generic-token-kubeconfig` secret contains the CA bundle for establishing trust to shoot API servers. However, as the secret is immutable its name changes with the rotation of the Cluster CA.
Extensions need to look up the `generic-token-kubeconfig.secret.gardener.cloud/name` annotation on the respective [`Cluster`](./cluster.md) object in order to determine which secret contains the current CA bundle.
The helper function `extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster` can be used for this task.

You can take a look at [CA Rotation in Extensions](./ca-rotation.md) for more details on the CA rotation feature in regard to extensions.

## How to create certificates for the shoot cluster?

Gardener creates several certificate authorities (CA) that are used to create server certificates for various components.
For example, the shoot's etcd has its own CA, the kube-aggregator has its own CA as well, and both are different to the actual cluster's CA.

With [GEP-18](../proposals/18-shoot-CA-rotation.md) (Shoot cluster CA rotation), extensions are required to do the same and generate dedicated CAs for their components (e.g. for signing a server certificate for cloud-controller-manager). They must not depend on the CA secrets managed by gardenlet.

Please see [CA Rotation in Extensions](./ca-rotation.md) for the exact requirements, that extensions need to fulfill in order to support the CA rotation feature.
