# `ManagedSeed`s: Register Shoot as Seed

An existing shoot can be registered as a seed by creating a `ManagedSeed` resource. This resource contains:

* The name of the shoot that should be registered as seed.
* A `gardenlet` section that contains:
  * `gardenlet` deployment parameters, such as the number of replicas, the image, etc.
  * The `GardenletConfiguration` resource that contains controllers configuration, feature gates, and a `seedConfig` section that contains the `Seed` spec and parts of its metadata.
  * Additional configuration parameters, such as the garden connection bootstrap mechanism (see [TLS Bootstrapping](../concepts/gardenlet.md#tls-bootstrapping)), and whether to merge the provided configuration with the configuration of the parent `gardenlet`.

`gardenlet` is deployed to the shoot, and it registers a new seed upon startup based on the `seedConfig` section.

> **Note:** Earlier Gardener allowed specifying a `seedTemplate` directly in the `ManagedSeed` resource. This feature is discontinued, any seed configuration must be via the `GardenletConfiguration`.

Note the following important aspects:

* Unlike the `Seed` resource, the `ManagedSeed` resource is namespaced. Currently, managed seeds are restricted to the `garden` namespace.
* The newly created `Seed` resource always has the same name as the `ManagedSeed` resource. Attempting to specify a different name in the `seedConfig` will fail.
* The `ManagedSeed` resource must always refer to an existing shoot. Attempting to create a `ManagedSeed` referring to a non-existing shoot will fail.
* A shoot that is being referred to by a `ManagedSeed` cannot be deleted. Attempting to delete such a shoot will fail.
* You can omit practically everything from the `gardenlet` section, including all or most of the `Seed` spec fields. Proper defaults will be supplied in all cases, based either on the most common use cases or the information already available in the `Shoot` resource.
* Also, if your seed is configured to host HA shoot control planes, then `gardenlet` will be deployed with multiple replicas across nodes or availability zones by default.
* Some `Seed` spec fields, for example the provider type and region, networking CIDRs for pods, services, and nodes, etc., must be the same as the corresponding `Shoot` spec fields of the shoot that is being registered as seed. Attempting to use different values (except empty ones, so that they are supplied by the defaulting mechanism) will fail.

## Deploying gardenlet to the Shoot

To register a shoot as a seed and deploy `gardenlet` to the shoot using a default configuration, create a `ManagedSeed` resource similar to the following:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeed
metadata:
  name: my-managed-seed
  namespace: garden
spec:
  shoot:
    name: crazy-botany
```

For an example that uses non-default configuration, see [55-managed-seed-gardenlet.yaml](../../example/55-managedseed-gardenlet.yaml)

### Renewing the Gardenlet Kubeconfig Secret

In order to make the `ManagedSeed` controller renew the gardenlet's kubeconfig secret, annotate the `ManagedSeed` with `gardener.cloud/operation=renew-kubeconfig`. This will trigger a reconciliation during which the kubeconfig secret is deleted and the bootstrapping is performed again (during which gardenlet obtains a new client certificate).

It is also possible to trigger the renewal on the secret directly, see [Rotate Certificates Using Bootstrap kubeconfig](../concepts/gardenlet.md#rotate-certificates-using-bootstrap-kubeconfig).

### Enforced Configuration Options

The following configuration options are enforced by Gardener API server for the ManagedSeed resources:

1. The vertical pod autoscaler should be enabled from the Shoot specification.

   The vertical pod autoscaler is a prerequisite for a Seed cluster. It is possible to enable the VPA feature for a Seed [(using the Seed spec)](./seed_settings.md#vertical-pod-autoscaler) and for a Shoot [(using the Shoot spec)](../usage/autoscaling/shoot_autoscaling.md#vertical-pod-auto-scaling). In context of `ManagedSeed`s, enabling the VPA in the Seed spec (instead of the Shoot spec) offers less flexibility and increases the network transfer and cost. Due to these reasons, the Gardener API server enforces the vertical pod autoscaler to be enabled from the Shoot specification.

1. The nginx-ingress addon should not be enabled for a Shoot referred by a ManagedSeed.

   An Ingress controller is also a prerequisite for a Seed cluster. For a Seed cluster, it is possible to enable Gardener managed Ingress controller or to deploy self-managed Ingress controller. There is also the nginx-ingress addon that can be enabled for a Shoot (using the Shoot spec). However, the Shoot nginx-ingress addon is in deprecated mode and it is not recommended for production clusters. Due to these reasons, the Gardener API server does not allow the Shoot nginx-ingress addon to be enabled for ManagedSeeds.
