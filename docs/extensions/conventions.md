# General Conventions

All the extensions that are registered to Gardener are deployed to the seed clusters on which they are required (also see [ControllerRegistration](controllerregistration.md)).

Some of these extensions might need to create global resources in the seed (e.g., `ClusterRole`s), i.e., it's important to have a naming scheme to avoid conflicts as it cannot be checked or validated upfront that two extensions don't use the same names.

Consequently, this page should help answering some general questions that might come up when it comes to developing an extension.

## Extension `Class`es

Each extension resource has a `class` (`spec.class`) field that is used to distinguish between different instances of the same extension type.
For extensions configured in `Shoot`s the class is named `shoot` (or unspecified for backwards compatibility), for `Seed`s the class is named `seed`.

Extension controllers ought to use the `class` field for event filtering (see [HasClass Predicate](https://github.com/gardener/gardener/blob/7361a19f4c3830a9f5134c073d3bfd72f4dcfa49/extensions/pkg/predicate/predicate.go#L60)) and during reconciliation.

## `PriorityClass`es

Extensions are not supposed to create and use self-defined `PriorityClasses`.
Instead, they can and should rely on well-known [`PriorityClasses`](../development/priority-classes.md) managed by gardenlet.

## High Availability of Deployed Components

Extensions might deploy components via `Deployment`s, `StatefulSet`s, etc., as part of the shoot control plane, or the seed or shoot system components.
In case a seed or shoot cluster is highly available, there are various failure tolerance types. For more information, see [Highly Available Shoot Control Plane](../usage/high-availability/shoot_high_availability.md).
Accordingly, the `replicas`, `topologySpreadConstraints` or `affinity` settings of the deployed components might need to be adapted.

Instead of doing this one-by-one for each and every component, extensions can rely on a mutating webhook provided by Gardener.
Please refer to [High Availability of Deployed Components](../development/high-availability-of-components.md) for details.

To reduce costs and to improve the network traffic latency in multi-zone clusters, extensions can make a Service topology-aware.
Please refer to [this document](../operations/topology_aware_routing.md) for details.

## Is there a naming scheme for (global) resources?

As there is no formal process to validate non-existence of conflicts between two extensions, please follow these naming schemes when creating resources (especially, when creating global resources, but it's in general a good idea for most created resources):

*The resource name should be prefixed with `extensions.gardener.cloud:<extension-type>-<extension-name>:<resource-name>`*, for example:

* `extensions.gardener.cloud:provider-aws:some-controller-manager`
* `extensions.gardener.cloud:extension-certificate-service:cert-broker`

## How to create resources in the shoot cluster?

Some extensions might not only create resources in the seed cluster itself but also in the shoot cluster. Usually, every extension comes with a `ServiceAccount` and the required RBAC permissions when it gets installed to the seed.
However, there are no credentials for the shoot for every extension.

Extensions are supposed to use [`ManagedResources`](../concepts/resource-manager.md#ManagedResource-controller) to manage resources in shoot clusters.
gardenlet deploys gardener-resource-manager instances into all shoot control planes, that will reconcile `ManagedResources` without a specified class (`spec.class=null`) in shoot clusters. Mind that Gardener acts on `ManagedResources` with the `origin=gardener` label. In order to prevent unwanted behavior, extensions should omit the `origin` label or provide their own unique value for it when creating such resources.

If you need to deploy a non-DaemonSet resource, Gardener automatically ensures that it only runs on nodes that are allowed to host system components and extensions. For more information, see [System Components Webhook](../concepts/resource-manager.md#System-Components-Webhook).

## How to create kubeconfigs for the shoot cluster?

Historically, Gardener extensions used to generate kubeconfigs with client certificates for components they deploy into the shoot control plane.
For this, they reused the shoot cluster CA secret (`ca`) to issue new client certificates.
With [gardener/gardener#4661](https://github.com/gardener/gardener/issues/4661) we moved away from using client certificates in favor of short-lived, auto-rotated `ServiceAccount` tokens. These tokens are managed by gardener-resource-manager's [`TokenRequestor`](../concepts/resource-manager.md#tokenrequestor).
Extensions are supposed to reuse this mechanism for requesting tokens and a `generic-token-kubeconfig` for authenticating against shoot clusters.

With [GEP-18](../proposals/18-shoot-CA-rotation.md) (Shoot cluster CA rotation), a dedicated CA will be used for signing client certificates ([gardener/gardener#5779](https://github.com/gardener/gardener/pull/5779)) which will be rotated when triggered by the shoot owner.
With this, extensions cannot reuse the `ca` secret anymore to issue client certificates.
Hence, extensions must switch to short-lived `ServiceAccount` tokens in order to support the CA rotation feature.

The `generic-token-kubeconfig` secret contains the CA bundle for establishing trust to shoot API servers. However, as the secret is immutable, its name changes with the rotation of the cluster CA.
Extensions need to look up the `generic-token-kubeconfig.secret.gardener.cloud/name` annotation on the respective [`Cluster`](./cluster.md) object in order to determine which secret contains the current CA bundle.
The helper function `extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster` can be used for this task.

You can take a look at [CA Rotation in Extensions](./ca-rotation.md) for more details on the CA rotation feature in regard to extensions.

## How to create certificates for the shoot cluster?

Gardener creates several certificate authorities (CA) that are used to create server certificates for various components.
For example, the shoot's etcd has its own CA, the kube-aggregator has its own CA as well, and both are different to the actual cluster's CA.

With [GEP-18](../proposals/18-shoot-CA-rotation.md) (Shoot cluster CA rotation), extensions are required to do the same and generate dedicated CAs for their components (e.g. for signing a server certificate for cloud-controller-manager). They must not depend on the CA secrets managed by gardenlet.

Please see [CA Rotation in Extensions](./ca-rotation.md) for the exact requirements that extensions need to fulfill in order to support the CA rotation feature.

## How to enforce a Pod Security Standard for extension namespaces?

The `pod-security.kubernetes.io/enforce` namespace label enforces the [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

You can set the `pod-security.kubernetes.io/enforce` label for extension namespace by adding the `security.gardener.cloud/pod-security-enforce` annotation to your `ControllerRegistration`. The value of the annotation would be the value set for the `pod-security.kubernetes.io/enforce` label. It is advised to set the annotation with the most restrictive pod security standard that your extension pods comply with.

If you are using the `./hack/generate-controller-registration.sh` script to generate your `ControllerRegistration` you can use the -e, --pod-security-enforce option to set the `security.gardener.cloud/pod-security-enforce` annotation. If the option is not set, it defaults to `baseline`.
