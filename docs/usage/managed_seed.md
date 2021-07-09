# Register Shoot as Seed

An existing shoot can be registered as a seed by creating a `ManagedSeed` resource. This resource replaces the `use-as-seed` annotation that was previously used to create [shooted seeds](shooted_seed.md), and that is already deprecated. It contains:

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

For an example that uses non-default configuration, see [55-managed-seed-seedtemplate.yaml](../../example/55-managedseed-seedtemplate.yaml)

## Migrating from the `use-as-seed` Annotation to `ManagedSeeds`

If you have existing seeds managed via the `use-as-seed` annotation, you should migrate them to `ManagedSeed` resources before support for the annotation has been completely removed from Gardener.

The *seed registration controller* that is responsible for reconciling the `use-as-seed` annotation is still functional, However, instead of reconciling the annotation directly as before, it converts it to a `ManagedSeed` resource and lets the *managed seed controller* perform the actual reconciliation. Therefore, for every `use-as-seed` annotation, you already have an equivalent `ManagedSeed` resource in your cluster. Since it has been created by reconciling an annotation on a shoot, it is also "owned" by the shoot, that is it contains an `ownerReference` to the corresponding shoot. This owner reference is used by the seed registration controller to determine that it should continue updating (or deleting) the `ManagedSeed` as a result of reconciling changes to (or the removal of) the `use-as-seed` annotation.

In order to migrate the `use-as-seed` annotation to a `ManagedSeed`, you should simply:

* Remove the owner reference to the shoot from the existing `ManagedSeed` resource.
* Remove the `use-as-seed` annotation from the `Shoot` resource.
* From this moment on, update or delete the `ManagedSeed` directly, instead of indirectly via the `use-as-seed` annotation.

If the shoot containing the `use-as-seed` annotation was created via a yaml file (e.g. via `kubectl apply -f`), a helm chart, or a script, you should update the corresponding file, template, or script so that it contains or generates the `ManagedSeed` that you have in your cluster, instead of the `use-as-seed` annotation. If you use an automated approach, make sure that the owner reference is removed from the existing `ManagedSeed` before removing the annotation from the `Shoot`.

### Specifying `apiServer` `replicas` and `autoscaler` options

A few of `use-as-seed` configuration options are not supported in a `Seed` resource, and therefore also not in a `ManagedSeed`. These options are (from the [shooted seeds](shooted_seed.md) description):

Option | Description
--- | ---
`apiServer.autoscaler.minReplicas` | Controls the minimum number of `kube-apiserver` replicas for the shooted seed cluster.
`apiServer.autoscaler.maxReplicas` | Controls the maximum number of `kube-apiserver` replicas for the shooted seed cluster.
`apiServer.replicas` | Controls how many `kube-apiserver` replicas the shooted seed cluster gets by default.

For backward compatibility, it is still possible to specify these options via the `shoot.gardener.cloud/managed-seed-api-server` annotation, using exactly the same syntax as before.

If you use any of these fields in any or your `use-as-seed` annotations, instead of removing the annotation completely as mentioned above, simply rename it to `managed-seed-api-server`, keeping these fields, and removing everything else.
