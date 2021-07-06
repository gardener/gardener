# Create Shooted Seed Cluster

Create managed seed (aka "shooted seed") cluster with the `shoot.gardener.cloud/use-as-seed` annotation.

**Note:** Starting with Gardener v1.18, the `shoot.gardener.cloud/use-as-seed` annotation is deprecated.
It still works as described here, however behind the scenes a `ManagedSeed` resource is created and reconciled.
It is strongly recommended to use such resources directly to register shoots as seeds, as described in [Register Shoot as Seed](managed_seed.md). For instructions how to migrate existing seeds managed via the `use-as-seed` annotation, see [Migrating from the `use-as-seed` Annotation to `ManagedSeeds`](managed_seed.md#migrating-from-the-use-as-seed-annotation-to-managedseeds).

## Procedure

1. Add the following project labels to the `garden` namespace if they don't exist yet:

    ```yaml
    labels:
      gardener.cloud/role: project
      project.gardener.cloud/name: garden
    ```

2. The annotation works only for shoot clusters created in the `garden` namespace. Create a project for the `garden` namespace using `kubectl` if you don't have one yet.

    > :warning:<br>Don't use the Gardener Dashboard as it would add a `garden` prefix for the namespace.

    Example: [/example/05-project-dev.yaml](../../example/05-project-dev.yaml)

    ```yaml
    apiVersion: core.gardener.cloud/v1beta1
    kind: Project
    metadata:
      name: garden
    spec:
      owner:
    ...
      namespace: garden
    ```

3. Create the shoot cluster.

    Set the following annotation on the `Shoot` to mark it as a shooted seed cluster.

    Example (full example: [/example/90-shoot.yaml](../../example/90-shoot.yaml)):

    ```yaml
      annotations:
        shoot.gardener.cloud/use-as-seed: >-
          true,shootDefaults.pods=100.96.0.0/11,shootDefaults.services=100.64.0.0/13,disable-capacity-reservation,with-secret-ref
    ```

    > * The networks from the seed cluster and its shoot clusters have to be different. To create shoot clusters with the dashboard you have to set a different worker CIDR in the shooted seed cluster (`spec.provider.infrastructureConfig` and `spec.networking.nodes`) and set the `shootDefaults` in the `shoot.gardener.cloud/use-as-seed` annotation to different CIDRs.
    > * Optional: The shoot clusters to be created can use the same network as the garden cluster. To use the same network, set different CIDRs for pods and services in the shooted seed cluster (`spec.networking.pods` and `spec.networking.services`).


## Configuration Options for the Seed Cluster



Option | Description
--- | ---
`true` | Registers the cluster as a seed cluster. Automatically deploys the gardenlet into the shoot cluster, unless specified otherwise (e.g. setting the `no-gardenlet` flag).
`no-gardenlet` | Prevents the automatic deployment of the gardenlet into the shoot cluster. Instead, the `Seed` object will be created with the assumption that another gardenlet will be responsible for managing it (according to its `seedConfig` configuration).
`disable-capacity-reservation` | Set `spec.settings.excessCapacity.enabled` in the seed cluster to false (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`invisible` | Set `spec.settings.scheduling.visible` in the seed cluster to false  (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`visible` | Set `spec.settings.scheduling.visible` in the seed cluster to true  (see [/example/50-seed.yaml](../../example/50-seed.yaml)) (**default**).
`disable-dns` | Set `spec.settings.shootDNS.enabled` in the seed cluster to false  (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`protected` | Only shoot clusters in the `garden` namespace can use this seed cluster.
`unprotected` | Shoot clusters from all namespaces can use this seed cluster (**default**).
`loadBalancerServices.annotations.*` | Set `spec.settings.loadBalancerServices.annotations` in the seed cluster (see [/example/50-seed.yaml](../../example/50-seed.yaml)), e.g `loadBalancerServices.annotations.service.beta.kubernetes.io/aws-load-balancer-type=nlb`.
`with-secret-ref` | Creates a secret with the `kubeconfig` of the cluster in the `garden` namespace in the garden cluster and specifies the `.spec.secretRef` in the `Seed` object accordingly.
`shootDefaults.pods` | Default pod network CIDR for shoot clusters created on this seed cluster.
`shootDefaults.services` | Default service network CIDR for shoot clusters created on this seed cluster.
`minimumVolumeSize` | Set `spec.volume.minimumSize` in the seed cluster (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`blockCIDRs` | Set `spec.network.blockCIDRs` seperated by `;` (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`backup.provider` | Set `spec.backup.provider` in the seed cluster (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`backup.region` | Set `spec.backup.region` in the seed cluster (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`backup.secretRef.name` | Set `spec.backup.secretRef.name` in the seed cluster (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`backup.secretRef.namespace` | Set `spec.backup.secretRef.namespace` in the seed cluster (see [/example/50-seed.yaml](../../example/50-seed.yaml)).
`apiServer.autoscaler.minReplicas` | Controls the minimum number of `kube-apiserver` replicas for the shooted seed cluster.
`apiServer.autoscaler.maxReplicas` | Controls the maximum number of `kube-apiserver` replicas for the shooted seed cluster.
`apiServer.replicas` | Controls how many `kube-apiserver` replicas the shooted seed cluster gets by default.
`use-serviceaccount-bootstrapping` | States that the gardenlet registers with the garden cluster using a temporary `ServiceAccount` instead of a `CertificateSigningRequest` (**default**)
`providerConfig.*` | Sets `providerConfig` configuration parameters of the Seed resource. Each parameter is specified via its path, e.g. `providerConfig.param1=foo` or `providerConfig.sublevel1.sublevel2.param3=bar`
`featureGates.*={true,false}` | Overwrites the `.featureGates` in the gardenlet configuration (only applicable when the `no-gardenlet` setting is **not** set), e.g. `featureGates.APIServerSNI=true`
`resources.capacity.*` | Overwrites the `resources.capacity` field in the gardenlet configuration (only applicable when the `no-gardenlet` setting is **not** set), e.g. `resources.capacity.shoots=250`
`resources.reserved.*` | Overwrites the `resources.reserved` field in the gardenlet configuration (only applicable when the `no-gardenlet` setting is **not** set), e.g. `resources.reserved.foo=42`
`ingress.controller.kind` | Activates and specifies the kind of the managed ingress controller in the seed
`ingress.controller.providerConfig.*` | Sets provider specific configuration parameters for the managed ingress controller. Each parameter is specified via its path, e.g. `ingress.controller.providerConfig.param1=foo` or `ingress.controller.providerConfig.sublevel1.sublevel2.param3=bar`
