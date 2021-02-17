# Register Shoot as Seed

An existing shoot can be registered as a seed by creating a `ManagedSeed` resource. This resource replaces the `use-as-seed` annotation that was previously used to create [shooted seeds](shooted_seed.md). It contains:

* The name of the shoot that should be registered as seed.
* An optional `seedTemplate` section that contains the `Seed` spec and parts of its metadata, such as labels and annotations.
* An optional `gardenlet` section that contains:
    * `gardenlet` deployment parameters, such as the number of replicas, the image, etc.
    * The `GardenletConfiguration` resource that contains controllers configuration, feature gates, and a `seedConfig` section that contains the `Seed` spec and parts of its metadata.
    * Additional configuration parameters, such as the garden connection bootstrap mechanism (see [TLS Bootstrapping](../concepts/gardenlet.md#tls-bootstrapping)), and whether to merge the provided configuration with the configuration of the parent `gardenlet`.
    
Either the `seedTemplate` or the `gardenlet` section must be specified, but not both: 

* If the `seedTemplate` section is specified, `gardenlet` is not deployed to the shoot, and a new `Seed` resource is created based on the template.
* If the `gardenlet` section is specified, `gardenlet` is deployed to the shoot, and it registers a new seed upon startup based on the `seedConfig` section of the `GardenletConfiguration` resource.

Note the following important aspects:

* Unlike the `Seed` resource, the `ManagedSeed` resource is namespaced. Currently, managed seeds are restricted to the `garden` namespace.
* The newly created `Seed` resource always has the same name as the `ManagedSeed` resource. Attempting to specify a different name in `seedTemplate` or `seedConfig` will fail. 
* The `ManagedSeed` resource must always refer to an existing shoot. Attempting to create a `ManagedSeed` referring to a non-existing shoot will fail.
* A shoot that is being referred to by a `ManagedSeed` cannot be deleted. Attempting to delete such a shoot will fail.  
* You can omit practically everything from the `seedTemplate` or `gardenlet` section, including all or most of the `Seed` spec fields. Proper defaults will be supplied in all cases, based either on the most common use cases or the information already available in the `Shoot` resource.
* Some `Seed` spec fields, for example the provider type and region, networking CIDRs for pods, services, and nodes, etc., must be the same as the corresponding `Shoot` spec fields of the shoot that is being registered as seed. Attempting to use different values (except empty ones, so that they are supplied by the defaulting mechanims) will fail.

## Deploying Gardenlet to the Shoot

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
  gardenlet: {}
```

For an example that uses non-default configuration, see [55-managed-seed-gardenlet.yaml](../../example/55-managedseed-gardenlet.yaml)

## Creating a Seed from a Template

To register a shoot as a seed from a template without deploying `gardenlet` to the shoot using a default configuration, create a `ManagedSeed` resource similar to the following:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeed
metadata:
  name: my-managed-seed
  namespace: garden
spec:
  shoot:
    name: crazy-botany
  seedTemplate:
    metadata:
      labels:
        seed.gardener.cloud/gardenlet: local
    spec:
      dns:
        ingressDomain: ""
      networks:
        pods: ""
        services: ""
      provider:
        type: ""
        region: ""
```

Note the `seed.gardener.cloud/gardenlet: local` label above. It stands for the label that is used in a `seedSelector` field of a `gardenlet` that takes care of multiple seeds. This label can be omitted if the `seedSelector` field selects all seeds. If there is no gardenlet running outside the cluster and selecting the seed, it won't be reconciled and no shoots will be scheduled on it.

For an example that uses non-default configuration, see [55-managed-seed-seedtemplate.yaml](../../example/55-managedseed-seedtemplate.yaml)