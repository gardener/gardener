# Automated Seed Management

Automated seed management involves automating certain aspects of managing seeds in Garden clusters, such as:

* Ensuring that the seeds capacity for shoots is not exceeded.
* Creating, deleting, and updating seeds declaratively (as "shooted seeds").
* Auto-scaling seeds upon reaching capacity thresholds.

Implementing the above features would involve changes to various existing Gardener components, as well as perhaps introducing new ones. This document describes these features in more detail and proposes a design approach for some of them.

In Gardener, scheduling shoots onto seeds is quite similar to scheduling pods onto nodes in Kubernetes. Therefore, a guiding principle behind the proposed design approaches is taking advantage of best practices and existing components already used in Kubernetes. 

## Ensuring Seeds Capacity for Shoots Is Not Exceeded

Seeds have a practical limit of how many shoots they can accommodate. Exceeding this limit is undesirable as the system performance will be noticeably impacted. Therefore, it is important to ensure that a seed's capacity for shoots is not exceeded by introducing a maximum number of shoots that can be scheduled onto a seed and making sure that it is taken into account by the scheduler. 

An initial discussion of this topic is available in [Issue #2938](https://github.com/gardener/gardener/issues/2938). The proposed solution is based on the following flow:

* The `gardenlet` is configured with certain *resources* and their total *capacity* (and, for certain resources, the amount reserved for Gardener).
* The `gardenlet` seed controller updates the Seed status with the capacity of each resource and how much of it is actually available to be consumed by shoots, using `capacity` and `allocatable` fields that are very similar to the corresponding fields in [the Node status](https://github.com/kubernetes/api/blob/2c3c141c931c0ab1ce1396c3152c72852b3d37ee/core/v1/types.go#L4582-L4593).
* When scheduling shoots, `gardener-scheduler` is influenced by the remaining capacity of the seed. In the simplest possible implementation, it never schedules shoots onto a seed that has already reached its capacity for a resource needed by the shoot.

Initially, the only resource considered would be the maximum number of shoots that can be scheduled onto a seed. Later, more resources could be added to make more precise scheduling calculations.

**Note:** Resources could also be requested by shoots, similarly to how pods can request node resources, and the scheduler could then ensure that such requests are taken into account when scheduling shoots onto seeds. However, the user is rarely, if at all, concerned with what  resources does a shoot consume from a seed, and this should also be regarded as an implementation detail that could change in the future. Therefore, such resource requests are not included in this GEP.

In addition, an extensibility plugin framework could be introduced in the future in order to advertise custom resources, including provider-specific resources, so that `gardenlet` would be able to update the seed status with their capacity and allocatable values, for example load balancers on Azure. Such a concept is not described here in further details as it is sufficiently complex to require a separate GEP. 

Example Seed status with `capacity` and `allocatable` fields:

```yaml
status:
  capacity:
    shoots: "100"
    persistent-volumes: "200" # Built-in resource
    azure.provider.extensions.gardener.cloud/load-balancers: "30" # Custom resource advertised by an Azure-specific plugin
  allocatable:
    shoots: "100"
    persistent-volumes: "197" # 3 persistent volumes are reserved for Gardener
    azure.provider.extensions.gardener.cloud/load-balancers: "300"
```

### Gardenlet Configuration

As mentioned above, the total resource capacity for built-in resources such as the number of shoots is specified as part of the `gardenlet` configuration, not in the Seed spec. The `gardenlet` configuration itself could be specified in the spec of the newly introduced [ShootedSeed](#shootedseeds) resource, which also contains the Seed template. Here it is assumed that in the future this could become the recommended and most widely used way to manage seeds. If the same `gardenlet` is responsible for multiple seeds, they would all share the same capacity settings.

To specify the total resource capacity for built-in resources, as well as the amount of such resources reserved for Gardener, the 2 new fields `resources.capacity` and `resources.reserved` are introduced in the `GardenletConfiguration` resource. The `gardenlet` seed controller would then initialize the `capacity` and `allocatable` fields in the seed status as follows:

* The `capacity` value is set to the configured `resources.capacity`.
* The `allocatable` value is set to the configured `resources.capacity` minus `resources.reserved`. 

Example `GardenletConfiguration` with `resources.capacity` and `resources.reserved` field:

```yaml
resources:
  capacity:
    shoots: 100
    persistent-volumes: 200
  reserved:
    persistent-volumes: 3
```

### Scheduling Algorithm

Currently `gardener-scheduler` uses a simple non-extensible algorithm in order to schedule shoots onto seeds. It goes through the following stages:

* Filter out seeds that don't meet scheduling requirements such as being ready, matching cloud profile and shoot label selectors, matching the shoot provider, and not having taints that are not tolerated by the shoot.
* From the remaining seeds, determine candidates that are considered best based on their region, by using a strategy that can be either "same region" or "minimal distance".
* Among these candidates, choose the one with the least number of shoots.

This scheduling algorithm should be adapted in order to properly take into account resources capacity and requests. As a first step, during the filtering stage, any seeds that would exceed their capacity for shoots, or their capacity for any resources requested by the shoot, should simply be filtered out and not considered during the next stages.

Later, the scheduling algorithm could be further enhanced by replacing the step in which the region strategy is applied by a scoring step similar to the one in [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/). In this scoring step, the scheduler would rank the remaining seeds to choose the most suitable shoot placement. It would assign a score to each seed that survived filtering based on a list of scoring rules. These rules might include for example `MinimalDistance` and `SeedResourcesLeastAllocated`, among others. Each rule would produce its own score for the seed, and the overall seed score would be calculated as a weighted sum of all such scores. Finally, the scheduler would assign the shoot to the seed with the highest ranking. 

## Creating, Deleting, and Updating Seeds Declaratively

When all or most of the existing seeds are near capacity, new seeds should be created in order to accommodate more shoots. Conversely, sometimes there could be too many seeds for the number of shoots, and so some of the seeds could be deleted to save resources. Currently, the process of creating a new seed involves a number of manual steps, such as creating a new shoot that meets certain criteria, and then registering it as a seed in Gardener. This could be automated to some extent by [annotating a shoot with the `use-as-seed` annotation](../usage/shooted_seed.md), in order to create a "shooted seed". However, adding more than one similar seeds still requires manually creating all needed shoots, annotating them appropriately, and making sure that they are successfully reconciled and registered.

To create, delete, and update seeds effectively in a declarative way and allow auto-scaling, a "creatable seed" resource along with a "set" (and in the future, perhaps also a "deployment") of such creatable seeds should be introduced, similar to Kubernetes `Pod`, `ReplicaSet`, and `Deployment` (or to MCM `Machine`, `MachineSet`, and `MachineDeployment`) resources. With such resources (and their respective controllers), creating a new seed based on a template would become as simple as increasing the `replicas` field in the "set" resource. 

In [Issue #2181](https://github.com/gardener/gardener/issues/2181) it is already proposed that the `use-as-seed` annotation is replaced by a dedicated `ShootedSeed` resource. The solution proposed here further elaborates on this idea.

### ShootedSeeds

#### ShootedSeed Resource

The `ShootedSeed` resource is a dedicated custom resource that represents a "shooted seed" and properly replaces the `use-as-seed` annotation. This resource contains:

* A Shoot template that contains the Shoot spec and parts of the metadata, such as labels and annotations.
* A Seed template that contains the Seed spec and parts of the metadata, such as labels and annotations.
* A `gardenlet` section that contains:
    * Whether `gardenlet` is enabled or not.
    * Certain aspects of the `gardenlet` deployment configuration, such as the number of replicas, the image, which bootstrap mechanism to use (bootstrap token / service account), etc.
    * The `GardenletConfiguration` resource that contains controllers configuration, feature gates, etc.
    
A ShootedSeed allows fine-tuning the seed and the `gardenlet` configuration of shooted seeds in order to deviate from the global defaults, e.g. lower the concurrent sync for some of the seed's controllers or enable a feature gate only on certain seeds. Also, it could simplify the deletion protection of shooted seeds. Last but not least, ShootedSeeds could be used as the basis for creating and deleting seeds automatically via the `ShootedSeedSet` resource that is described in more details below.

Unlike the `Seed` resource, the `ShootedSeed` resource is namespaced. If created in the `garden` namespace, the resulting seeds are globally available. If created in a project namespace, the resulting seeds can be used as "private seeds" by shoots in the project, either by being decorated with project-specific taints and labels, or by being of the special `PrivateSeed` kind that is also namespaced. The concept of private seeds / cloudprofiles is described in [Issue #2874](https://github.com/gardener/gardener/issues/2874). Until this concept is implemented, `ShootedSeed` resources might need to be restricted to the `garden` namespace, similarly to how shoots with the `use-as-seed` annotation currently are.

Example `ShootedSeed` resource:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ShootedSeed
metadata:
  name: crazy-botany
  namespace: garden
spec:
  shootTemplate: # Shoot template, including spec and parts of the metadata
    metadata:
      labels:
        foo: bar
    spec:
      cloudProfileName: gcp
      secretBindingName: shoot-operator-gcp
      region: europe-west1
      provider:
        type: gcp
      ...
  seedTemplate: # Seed template, including spec and parts of the metadata
    metadata:
      labels:
        foo: bar
    spec:
      provider:
        type: gcp
        region: europe-west1
      taints:
      - key: seed.gardener.cloud/protected
      ...
  gardenlet: 
    enabled: true
    deployment: # Gardenlet deployment configuration
      replicas: 1
      revisionHistoryLimit: 10
      serviceAccountName: gardenlet
      image:
        repository: eu.gcr.io/gardener-project/gardener/gardenlet
        tag: latest
        pullPolicy: IfNotPresent
      resources:
        ...
      annotations: 
        ...
      labels:
        ...
      env:
        ...
      imageVectorOverwrite: | # See docs/deployment/image_vector.md
        ...
      componentImageVectorOverwrites: | # See docs/deployment/image_vector.md
        ...
      dnsConfig:
        ...
      additionalVolumes:
        ...
      additionalVolumeMounts:
        ...
      vpa: false
    config: # GardenletConfiguration resource
      controllers:
        shoot:
          concurrentSyncs: 20
      featureGates:
        CachedRuntimeClients: true
      ...
```

#### ShootedSeed Controller

ShootedSeeds are reconciled by a new *shooted seed controller* in `gardenlet`. During the reconciliation this controller creates, deletes, and updates Shoots, Seeds, and `gardenlet` Deployments in shooted seeds as needed. Its implementation is similar to the current [seed registration controller](https://github.com/gardener/gardener/blob/master/pkg/gardenlet/controller/shoot/seed_registration_control.go), with the difference that it reconciles the new `ShootedSeed` resource instead of  

Once the `ShootedSeed` resource and its controller are considered sufficiently stable, the current `use-as-seed` annotation and the controller mentioned above should be marked as deprecated and eventually removed.

**Note:** Bootstrapping a new seed might require additional provider-specific actions to the ones performed automatically by the shooted seed controller. For example, on Azure this might include getting a new subscription, extending quotas, etc. This could eventually be automated by introducing an extension mechanism for the Gardener seed bootstrapping flow, to be handled by a new type of controller in the provider extensions. However, such an extension mechanism is not in the scope of this proposal and might require a separate GEP.

### ShootedSeedSets

Similarly to a [ReplicaSet](https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/), the purpose of a ShootedSeedSet is to maintain a stable set of replica [ShootedSeeds](#shootedseeds) available at any given time. As such, it is used to guarantee the availability of a specified number of identical ShootedSeeds.

#### ShootedSeedSet Resource

The `ShootedSeedSet` resource has a `selector` field that specifies how to identify ShootedSeeds it can acquire, a number of `replicas` indicating how many ShootedSeeds it should be maintaining, and a shooted seed `template` specifying the data of new ShootedSeeds it should create to meet the number of replicas criteria. A ShootedSeedSet then fulfills its purpose by creating and deleting ShootedSeeds as needed to reach the desired number. 

A ShootedSeedSet is linked to its ShootedSeeds via the ShootedSeeds' `metadata.ownerReferences` field, which specifies what resource the current object is owned by. All ShootedSeeds acquired by a ShootedSeedSet have their owning ShootedSeedSet's identifying information within their `ownerReferences` field. 

Example `ShootedSeedSet` resource:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ShootedSeedSet
metadata:
  name: crazy-botany
  namespace: garden
spec:
  replicas: 1
  selector:
    matchLabels:
      foo: bar
  template:
    metadata:
      labels:
        foo: bar
    spec: # ShootedSeed resource
      shootTemplate: 
        ...
      seedTemplate: 
        ...
      gardenlet: 
        ...
```

#### ShootedSeedSet Controller

ShootedSeedSets are reconciled by a new *shooted seed set controller* in `gardenlet`. During the reconciliation this controller creates and deletes ShootedSeeds in response to changes to the `replicas` and `selector` fields.

#### ShootedSeedDeployments

A ShootedSeedSet, similarly to a `ReplicaSet`, does not manage updates to its replicas in any way. In the future, we might introduce ShootedSeedDeployments, a higher-level concept that manages ShootedSeedSets and provides declarative updates to ShootedSeeds along with other useful features, similarly to a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/). However, there is an important difference between seeds and pods or nodes in that seeds are more "heavyweight" and therefore updating a set of seeds by introducing new seeds and moving shoots to them tends to be much more complex, time-consuming, and prone to failures compared to updating the seeds "in place". Furthermore, updating seeds in this way depends on a mature implementation of [GEP-7: Shoot Control Plane Migration](07-shoot-control-plane-migration.md), which is not available right now. Therefore, ShootedSeedDeployments are not described in detail here, as properly dealing with the challenges mentioned requires a separate GEP.

#### Deleting ShootedSeeds

Deleting ShootedSeeds in response to decreasing the replicas of a ShootedSeedSet deserves special attention for two reasons:

* A seed that is already in use by shoots cannot be deleted, unless the shoots are either deleted or moved to other seeds first.
* When there are more empty seeds than requested for deletion, determining which seeds to delete might not be as straightforward as with pods or nodes, if the seeds are based on different versions of the template and therefore not completely identical.

The above challenges could be addressed as follows:

* In order to scale in a ShootedSeedSet successfully, there should be at least as many empty ShootedSeeds as the difference between the old and the new replicas. In some cases, the user might need to ensure that this is the case by draining some seeds manually before decreasing the replicas field.
* It should be possible to protect ShootedSeeds from deletion even if they are empty, perhaps via an annotation such as `gardener.cloud/protected`. Such seeds are not taken into account when determining whether the scale in operation can succeed.
* The decision which seeds to delete among the seeds that are empty and not protected should be based on a set of heuristics and hints, perhaps again in the form of annotations, that could be added manually by the user. 

## Auto-scaling Seeds

The most interesting and advanced automated seed management feature is making sure that a Garden cluster has enough seeds registered to schedule new shoots (and, in the future, reschedule shoots from drained seeds) without exceeding the seeds capacity for shoots, but not more than actually needed at any given moment. This would involve introducing an auto-scaling mechanism for seeds in Garden clusters. 

The proposed solution builds upon the ideas introduced earlier. The [`ShootedSeedSet`](#shootedseeds) resource (and in the future, also the `ShootedSeedDeployment` resource) could have a `scale` subresource that changes the `replicas` field. This would allow a new "seed autoscaler" controller to scale these resources via a special "autoscaler" resource (for example `SeedAutoscaler`), similarly to how the Kubernetes [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) controller scales pods, as described in [Horizontal Pod Autoscaler Walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/). 

The primary metric used for scaling should be the number of shoots already scheduled onto that seed either as a direct value or as a percentage of the seed's capacity for shoots introduced in [Ensuring Seeds Capacity for Shoots Is Not Exceeded](#ensuring-seeds-capacity-for-shoots-is-not-exceeded) (*utilization*). Later, custom metrics based on other resources, including provider-specific resources, could be considered as well.

**Note:** Even if the controller is called *Horizontal Pod Autoscaler*, it is capable of scaling any resource with a `scale` subresource, using any custom metric. Therefore, initially it was proposed to use this controller directly. However, a number of important drawbacks were identified with this approach, and so it is no longer proposed here.

### SeedAutoscaler

The SeedAutoscaler automatically scales the number of [ShootedSeeds](#shootedseeds) in a [ShootedSeedSet](#shootedseedsets) based on observed resource utilization. The resource could be any resource that is tracked via the `capacity` and `allocatable` fields in the Seed status, including in particular the number of shoots already scheduled onto the seed.

#### SeedAutoscaler Resource

The SeedAutoscaler is implemented as a custom resource and a new controller. The resource determines the behavior of the controller. The `SeedAutoscaler` resource has a `scaleTargetRef` that specifies the target resource to be scaled, the minimum and maximum number of replicas, as well as a list of metrics. The only supported metric type initially is `Resource` for resources that are tracked via the `capacity` and `allocatable` fields in the Seed status. The resource target can be of type `Utilization` or `AverageValue`.

Example `SeedAutoscaler` resource:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: SeedAutoscaler
metadata:
  name: crazy-botany
  namespace: garden
spec:
  scaleTargetRef:
    apiVersion: core.gardener.cloud/v1beta1
    kind: ShootedSeedSet
    name: crazy-botany
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource # Only Resource is supported
    resource:
      name: shoots
      target:
        type: Utilization # Utilization or AverageValue
        averageUtilization: 50
```

#### SeedAutoscaler Controller

`SeedAutoscaler` resources are reconciled by a new controller, either in `gardenlet` or out-of-tree, similarly to [cluster-autoscaler](https://github.com/gardener/autoscaler). The controller periodically adjusts the number of replicas in a ShootedSeedSet to match the observed average resource utilization to the target specified by user.

**Note:** The SeedAutoscaler controller is not limited to evaluating only metrics, it could also evaluate taints, label selectors, etc. This is not yet reflected in the example `SeedAutoscaler` resource below. Such details are intentionally not specified in this GEP, they should be further explored in the issues created to track the actual implementation.
