# Automated Seed Management

Automated seed management involves automating certain aspects of managing seeds in Garden clusters, such as:

* [Ensuring that the seeds capacity for shoots is not exceeded](#ensuring-seeds-capacity-for-shoots-is-not-exceeded)
* [Creating, deleting, and updating seeds declaratively as "managed seeds"](#managedseeds)
* [Declaratively managing sets of similar "managed seeds" as "managed seed sets" which can be scaled up/down](#managedseedsets)
* [Auto-scaling seeds upon reaching capacity thresholds](#auto-scaling-seeds)

Implementing the above features would involve changes to various existing Gardener components, as well as perhaps introducing new ones. This document describes these features in more detail and proposes a design approach for some of them.

In Gardener, scheduling shoots onto seeds is quite similar to scheduling pods onto nodes in Kubernetes. Therefore, a guiding principle behind the proposed design approaches is taking advantage of best practices and existing components already used in Kubernetes. 

## Ensuring Seeds Capacity for Shoots Is Not Exceeded

Seeds have a practical limit of how many shoots they can accommodate. Exceeding this limit is undesirable, as the system performance will be noticeably impacted. Therefore, it is important to ensure that a seed's capacity for shoots is not exceeded by introducing a maximum number of shoots that can be scheduled onto a seed and making sure that it is taken into account by the scheduler. 

An initial discussion of this topic is available in [Issue #2938](https://github.com/gardener/gardener/issues/2938). The proposed solution is based on the following flow:

* The `gardenlet` is configured with certain *resources* and their total *capacity* (and, for certain resources, the amount reserved for Gardener).
* The `gardenlet` seed controller updates the Seed status with the capacity of each resource and how much of it is actually available to be consumed by shoots, using `capacity` and `allocatable` fields that are very similar to the corresponding fields in [the Node status](https://github.com/kubernetes/api/blob/2c3c141c931c0ab1ce1396c3152c72852b3d37ee/core/v1/types.go#L4582-L4593).
* When scheduling shoots, `gardener-scheduler` is influenced by the remaining capacity of the seed. In the simplest possible implementation, it never schedules shoots onto a seed that has already reached its capacity for a resource needed by the shoot.

Initially, the only resource considered would be the maximum number of shoots that can be scheduled onto a seed. Later, more resources could be added to make more precise scheduling calculations.

> **Note:** Resources could also be requested by shoots, similarly to how pods can request node resources, and the scheduler could then ensure that such requests are taken into account when scheduling shoots onto seeds. However, the user is rarely, if at all, concerned with what resources a shoot consumes from a seed, and this should also be regarded as an implementation detail that could change in the future. Therefore, such resource requests are not included in this GEP.

In addition, an extensibility plugin framework could be introduced in the future in order to advertise custom resources, including provider-specific resources, so that the `gardenlet` would be able to update the seed status with their capacity and allocatable values, for example, load balancers on Azure. Such a concept is not described here in further details as it is sufficiently complex to require a separate GEP. 

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

### gardenlet Configuration

As mentioned above, the total resource capacity for built-in resources such as the number of shoots is specified as part of the `gardenlet` configuration, not in the Seed spec. The `gardenlet` configuration itself could be specified in the spec of the newly introduced [ManagedSeed](#managedseeds) resource. Here it is assumed that in the future this could become the recommended and most widely used way to manage seeds. If the same `gardenlet` is responsible for multiple seeds, they would all share the same capacity settings.

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

Later, the scheduling algorithm could be further enhanced by replacing the step in which the region strategy is applied by a scoring step similar to the one in [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/). In this scoring step, the scheduler would rank the remaining seeds to choose the most suitable shoot placement. It would assign a score to each seed that survived filtering based on a list of scoring rules. These rules might include, for example, `MinimalDistance` and `SeedResourcesLeastAllocated`, among others. Each rule would produce its own score for the seed, and the overall seed score would be calculated as a weighted sum of all such scores. Finally, the scheduler would assign the shoot to the seed with the highest ranking. 

## ManagedSeeds

When all or most of the existing seeds are near capacity, new seeds should be created in order to accommodate more shoots. Conversely, sometimes there could be too many seeds for the number of shoots, and so some of the seeds could be deleted to save resources. Currently, the process of creating a new seed involves a number of manual steps, such as creating a new shoot that meets certain criteria, and then registering it as a seed in Gardener. This could be automated to some extent by annotating a shoot with the `use-as-seed` annotation, in order to create a "shooted seed". However, adding more than one similar seeds still requires manually creating all needed shoots, annotating them appropriately, and making sure that they are successfully reconciled and registered.

To create, delete, and update seeds effectively in a declarative way and allow auto-scaling, a "creatable seed" resource along with a "set" (and in the future, perhaps also a "deployment") of such creatable seeds should be introduced, similar to Kubernetes `Pod`, `ReplicaSet`, and `Deployment` (or to MCM `Machine`, `MachineSet`, and `MachineDeployment`) resources. With such resources (and their respective controllers), creating a new seed based on a template would become as simple as increasing the `replicas` field in the "set" resource. 

In [Issue #2181](https://github.com/gardener/gardener/issues/2181), it is already proposed that the `use-as-seed` annotation is replaced by a dedicated `ShootedSeed` resource. The solution proposed here further elaborates on this idea.

### ManagedSeed Resource

The `ManagedSeed` resource is a dedicated custom resource that represents an evolution of the "shooted seed" and properly replaces the `use-as-seed` annotation. This resource contains:

* The name of the Shoot that should be registered as a Seed.
* An optional `seedTemplate` section that contains the Seed spec and parts of the metadata, such as labels and annotations.
* An optional `gardenlet` section that contains:
    * `gardenlet` deployment parameters, such as the number of replicas, the image, etc.
    * The `GardenletConfiguration` resource that contains controller configurations, feature gates, and a `seedConfig` section that contains the `Seed` spec and parts of its metadata.
    * Additional configuration parameters, such as the garden connection bootstrap mechanism (see [TLS Bootstrapping](../concepts/gardenlet.md#tls-bootstrapping)), and whether to merge the provided configuration with the configuration of the parent `gardenlet`.
    
Either the `seedTemplate` or the `gardenlet` section must be specified, but not both:

* If the `seedTemplate` section is specified, the `gardenlet` is not deployed to the shoot, and a new `Seed` resource is created based on the template.
* If the `gardenlet` section is specified, the `gardenlet` is deployed to the shoot, and it registers a new seed upon startup based on the `seedConfig` section of the `GardenletConfiguration` resource.

A ManagedSeed allows fine-tuning the seed and the `gardenlet` configuration of shooted seeds in order to deviate from the global defaults, e.g. lower the concurrent sync for some of the seed's controllers or enable a feature gate only on certain seeds. Also, it simplifies the deletion protection of such seeds. 

Also, the `ManagedSeed` resource is a more powerful alternative to the `use-as-seed` annotation. The implementation of the `use-as-seed` annotation itself could be refactored to use a `ManagedSeed` resource extracted from the annotation by a controller.
    
Although in this proposal a ManagedSeed is always a "shooted seed", that is a Shoot that is registered as a Seed, this idea could be further extended in the future by adding a `type` field that could be either `Shoot` (implied in this proposal), or something different. Such an extension would allow to register and manage as Seed a cluster that is not a Shoot, e.g., a GKE cluster.

Last but not least, ManagedSeeds could be used as the basis for creating and deleting seeds automatically via the `ManagedSeedSet` resource that is described in [ManagedSeedSets](#managedseedsets).

Unlike the `Seed` resource, the `ManagedSeed` resource is namespaced. If created in the `garden` namespace, the resulting seed is globally available. If created in a project namespace, the resulting seed can be used as a "private seed" by shoots in the project, either by being decorated with project-specific taints and labels, or by being of the special `PrivateSeed` kind that is also namespaced. The concept of private seeds / cloudprofiles is described in [Issue #2874](https://github.com/gardener/gardener/issues/2874). Until this concept is implemented, `ManagedSeed` resources might need to be restricted to the `garden` namespace, similarly to how shoots with the `use-as-seed` annotation currently are.

Example `ManagedSeed` resource with a `seedTemplate` section:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeed
metadata:
  name: crazy-botany
  namespace: garden
spec:
  shoot:
    name: crazy-botany # Shoot that should be registered as a Seed
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
```

Example `ManagedSeed` resource with a `gardenlet` section:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeed
metadata:
  name: crazy-botany
  namespace: garden
spec:
  shoot:
    name: crazy-botany # Shoot that should be registered as a Seed
  gardenlet: 
    deployment: # Gardenlet deployment configuration
      replicaCount: 1
      revisionHistoryLimit: 10
      serviceAccountName: gardenlet
      image:
        repository: eu.gcr.io/gardener-project/gardener/gardenlet
        tag: latest
        pullPolicy: IfNotPresent
      resources:
        ...
      podLabels:
        ...
      podAnnotations: 
        ...
      additionalVolumes:
        ...
      additionalVolumeMounts:
        ...
      env:
        ...
      vpa: false
    config: # GardenletConfiguration resource
      apiVersion: gardenlet.config.gardener.cloud/v1alpha1
      kind: GardenletConfiguration
      seedConfig: # Seed template, including spec and parts of the metadata
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
      controllers:
        shoot:
          concurrentSyncs: 20
      featureGates:
        ...
      ...
    bootstrap: BootstrapToken
    mergeWithParent: true
```

### ManagedSeed Controller

ManagedSeeds are reconciled by a new *managed seed controller* in the `gardenlet`. Its implementation is very similar to the current seed registration controller, and in fact could be regarded as a refactoring of the latter, with the difference that it uses the `ManagedSeed` resource rather than the `use-as-seed` annotation on a Shoot. The `gardenlet` only reconciles ManagedSeeds that refer to Shoots scheduled on Seeds the `gardenlet` is responsible for.

Once this controller is considered sufficiently stable, the current `use-as-seed` annotation and the controller mentioned above should be marked as deprecated and eventually removed.

A `ManagedSeed` that is in use by shoots cannot be deleted unless the shoots are either deleted or moved to other seeds first. The managed seed controller ensures that this is the case by only allowing a ManagedSeed to be deleted if its Seed has been already deleted.

### ManagedSeed Admission Plugins

In addition to the managed seed controller mentioned above, new `gardener-apiserver` admission plugins should be introduced to properly validate the creation and update of ManagedSeeds, as well as the deletion of shoots registered as seeds. These plugins should ensure that: 

* A `Shoot` that is being referred to by a `ManagedSeed` cannot be deleted.   
* Certain `Seed` spec fields, for example the provider type and region, networking CIDRs for pods, services, and nodes, etc., are the same as (or compatible with) the corresponding `Shoot` spec fields of the shoot that is being registered as seed. 
* If such `Seed` spec fields are omitted or empty, the plugins should supply proper defaults based on the values in the `Shoot` resource.

### Provider-specific Seed Bootstrapping Actions

Bootstrapping a new seed might require additional provider-specific actions to the ones performed automatically by the managed seed controller. For example, on Azure this might include getting a new subscription, extending quotas, etc. This could eventually be automated by introducing an extension mechanism for the Gardener seed bootstrapping flow, to be handled by a new type of controller in the provider extensions. However, such an extension mechanism is not in the scope of this proposal and might require a separate GEP.

One idea that could be further explored is the use *shoot readiness gates*, similar to Kubernetes [pod readiness gates](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-readiness-gate), in order to control whether a Shoot is considered `Ready` before it could be registered as a Seed. A provider-specific extension could set the special condition that is specified as a readiness gate to `True` only after it has successfully performed the provider-specific actions needed.

### Changes to Existing Controllers

Since the Shoot registration as a Seed is decoupled from the Shoot reconciliation, existing `gardenlet` controllers would not have to be changed in order to properly support ManagedSeeds. The main change to `gardenlet` that would be needed is introducing the new *managed seed controller* mentioned above, and possibly retiring the old one at some point. In addition, the Shoot controller would need to be adapted as it currently performs certain actions differently if the shoot has a "shooted seed".

The introduction of the `ManagedSeed` resource would also require no changes to existing `gardener-controller-manager` controllers that operate on Shoots (for example, shoot hibernation and maintenance controllers).

## ManagedSeedSets

Similarly to a [ReplicaSet](https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/), the purpose of a ManagedSeedSet is to maintain a stable set of replica [ManagedSeeds](#managedseeds) available at any given time. As such, it is used to guarantee the availability of a specified number of identical ManagedSeeds, on an equal number of identical Shoots.

### ManagedSeedSet Resource

The `ManagedSeedSet` resource has a `selector` field that specifies how to identify ManagedSeeds it can acquire, a number of `replicas` indicating how many ManagedSeeds (and their corresponding Shoots) it should be maintaining, and a two templates:

* A ManagedSeed template (`template`) specifying the data of new ManagedSeeds it should create to meet the number of replicas criteria.
* A Shoot template (`shootTemplate`) specifying the data of new Shoots it should create to host the ManagedSeeds. 

A ManagedSeedSet then fulfills its purpose by creating and deleting ManagedSeeds (and their corresponding Shoots) as needed to reach the desired number. 

A ManagedSeedSet is linked to its ManagedSeeds and Shoots via the `metadata.ownerReferences` field, which specifies what resource the current object is owned by. All ManagedSeeds and Shoots acquired by a ManagedSeedSet have their owning ManagedSeedSet's identifying information within their `ownerReferences` field. 

Example `ManagedSeedSet` resource:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeedSet
metadata:
  name: crazy-botany
  namespace: garden
spec:
  replicas: 3
  selector:
    matchLabels:
      foo: bar
  updateStrategy:
    type: RollingUpdate # Update strategy, must be `RollingUpdate`
    rollingUpdate:
      partition: 2 # Only update the last replica (#2), assuming there are no gaps ("rolling out a canary")
  template: # ManagedSeed template, including spec and parts of the metadata
    metadata:
      labels:
        foo: bar
    spec: 
      # shoot.name is not specified since it's filled automatically by the controller
      seedTemplate: # Either a seed or a gardenlet section must be specified, see above
        metadata:
          labels:
            foo: bar
        provider:
          type: gcp
          region: europe-west1
        taints:
        - key: seed.gardener.cloud/protected
        ...
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
```

### ManagedSeedSet Controller

ManagedSeedSets are reconciled by a new *managed seed set controller* in `gardener-controller-manager`. During the reconciliation this controller creates and deletes ManagedSeeds and Shoots in response to changes to the `replicas` and `selector` fields.

> **Note:** The introduction of the `ManagedSeedSet` resource would not require any changes to `gardenlet` or to existing `gardener-controller-manager` controllers. 

### Managing ManagedSeed Updates

To manage ManagedSeed updates, we considered two possible approaches:

* A ManagedSeedSet, similarly to a ReplicaSet, does not manage updates to its replicas in any way. In the future, we might introduce ManagedSeedDeployments, a higher-level concept that manages ManagedSeedSets and provides declarative updates to ManagedSeeds along with other useful features, similarly to a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/). Such a mechanism would involve creating new ManagedSeedSets, and therefore new seeds, behind the scenes, and moving existing shoots to them.
* A ManagedSeedSet does manage updates to its replicas, similarly to a [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/). Updates are performed "in-place", without creating new seeds and moving existing shoots to them. Such a mechanism could also take advantage of other StatefulSet features, such as ordered rolling updates and phased rollouts. 

There is an important difference between seeds and pods or nodes in that seeds are more "heavyweight" and therefore updating a set of seeds by introducing new seeds and moving shoots to them tends to be much more complex, time-consuming, and prone to failures compared to updating the seeds "in place". Furthermore, updating seeds in this way depends on a mature implementation of [GEP-7: Shoot Control Plane Migration](07-shoot-control-plane-migration.md), which is not available right now. Due to these considerations, we favor the second approach over the first one.

#### ManagedSeed Identity and Order

A StatefulSet manages the deployment and scaling of a set of Pods, and provides guarantees about the ordering and uniqueness of these Pods. It maintains a *stable identity* (including network identity) for each of their Pods. These pods are created from the same spec, but are not interchangeable: each has a persistent identifier that it maintains across any rescheduling.

A StatefulSet achieves the above by associating each replica with an *ordinal number*. With n replicas, these ordinal numbers range from 0 to n-1. When scaling out, newly added replicas always have ordinal numbers larger than those of previously existing replicas. When scaling in, it is the replicas with the largest original numbers that are removed.

Besides stable identity and persistent storage, these ordinal numbers are also used to implement the following StatefulSet features: 

* Ordered, graceful deployment and scaling.
* Ordered, automated rolling updates. Such rolling updates can be *partitioned* (limited to replicas with ordinal numbers greater than or equal to the "partition") to achieve *phased rollouts*.

A ManagedSeedSet, unlike a StatefulSet, does not need to maintain a stable identity for its ManagedSeeds. Furthermore, it would not be practical to always remove the replicas with the largest ordinal numbers when scaling in, since the corresponding seeds may have shoots scheduled onto them, while other seeds, with lower ordinals, may have fewer shoots (or none), and therefore be much better candidates for being removed.

On the other hand, it would be beneficial if a ManagedSeedSet, like a StatefulSet, provides ordered deployment and scaling, ordered rolling updates, and phased rollouts. The main advantage of these features is that a deployment or update failure would affect fewer replicas (ideally just one), containing any potential damage and making the situation easier to handle, thus achieving some of the goals stated in [Issue #87](https://github.com/gardener/gardener/issues/87). They could also help to contain seed rolling updates outside business hours.

Based on the above considerations, we propose the following mechanism for handling ManagedSeed identity and order:

* A ManagedSeedSet uses *ordinal numbers generated by an increasing sequence* to identify ManagedSeeds and Shoots it creates and manages. These numbers always start from 0 and are incremented by 1 for each newly added replica. 
* Replicas (both ManagedSeeds and Shoots) are named after the ManagedSeedSet with the ordinal number appended. For example, for a ManagedSeedSet named `test` its replicas are named `test-0`, `test-1`, etc.
* Gaps in the sequence created by removing replicas with ordinal numbers in the middle of the range are never filled in. A newly added replica always receives a number that is not only free, but also unique to itself. For example, if there are 2 replicas named `test-0` and `test-1` and any one of them is removed, a newly added replica will still be named `test-2`.   

Although such ordinal numbers can also provide some form of stable identity, in this case it is much more important that they can provide a predictable ordering for deployments and updates, and can also be used to partition rolling updates similar to StatefulSet ordinal numbers.

#### Update Strategies

The ManagedSeedSet's `.spec.updateStrategy` field allows configuring automated rolling updates for the ManagedSeeds and Shoots in a ManagedSeedSet.

**Rolling Updates**

The `RollingUpdate` update strategy implements automated, rolling update for the ManagedSeeds and Shoots in a ManagedSeedSet. With this strategy, the ManagedSeedSet controller will update each ManagedSeed and Shoot in the ManagedSeedSet. It will proceed from the largest number to the smallest, updating each ManagedSeed and its corresponding Shoot one at a time. It will wait until both the Shoot and the Seed of an updated ManagedSeed are Ready prior to updating its predecessor.

As a further improvement upon the above, the controller could check not only the ManagedSeeds and their corresponding Shoots for readiness, but also the Shoots scheduled onto these ManagedSeeds. The rollout would then only continue if no more than X percent of these Shoots are not reconciled and Ready. Since checking all these additional conditions might require some complex logic, it should be performed by an independent *managed seed care controller* that updates the ManagedSeed resource with the readiness of its Seed and all Shoots scheduled onto the Seed.

Note that unlike a StatefulSet, an `OnDelete` update strategy is not supported.

**Partitions**

The `RollingUpdate` update strategy can be partitioned by specifying a `.spec.updateStrategy.rollingUpdate.partition`. If a partition is specified, only ManagedSeeds and Shoots with ordinals greater than or equal to the partition will be updated when any of the ManagedSeedSet's templates is updated. All remaining ManagedSeeds and Shoots will not be updated. If a ManagedSeedSet's `.spec.updateStrategy.rollingUpdate.partition` is greater than the largest ordinal number in use by a replica, updates to its templates will not be propagated to its replicas (but newly added replicas may still use the updated templates depending on the partition value).

#### Keeping Track of Revision History and Performing Rollbacks

Similarly to a StatefulSet, the ManagedSeedSet controller uses [ControllerRevisions](https://pkg.go.dev/k8s.io/api/apps/v1#ControllerRevision) to keep track of the revision history, and `controller-revision-hash` labels to maintain an association between a ManagedSeed or a Shoot and the concrete template revisions based on which they were created or last updated. These are used for the following purposes:

* During an update, determine which replicas are still not on the latest revision and therefore should be updated.
* Display the revision history of a ManagedSeedSet via `kubectl rollout history`.
* Roll back all ManagedSeedSet replicas to a specific revision via `kubectl rollout undo`

> **Note:** The above `kubectl rollout` commands will not work with custom resources such as ManagedSeedSets out of the box (the [documentation](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#rollout) says explicitly that valid resource types are only deployments, daemonsets, and statefulsets), but it should be possible to eventually support such commands for ManagedSeedSets via a [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).

### Scaling-in ManagedSeedSets

Deleting ManagedSeeds in response to decreasing the replicas of a ManagedSeedSet deserves special attention for two reasons:

* A seed that is already in use by shoots cannot be deleted, unless the shoots are either deleted or moved to other seeds first.
* When there are more empty seeds than requested for deletion, determining which seeds to delete might not be as straightforward as with pods or nodes.

The above challenges could be addressed as follows:

* In order to scale in a ManagedSeedSet successfully, there should be at least as many empty ManagedSeeds as the difference between the old and the new replicas. In some cases, the user might need to ensure that this is the case by draining some seeds manually before decreasing the replicas field.
* It should be possible to protect ManagedSeeds from deletion even if they are empty, perhaps via an annotation such as `seedmanagement.gardener.cloud/protect-from-deletion`. Such seeds are not taken into account when determining whether the scale in operation can succeed.
* The decision which seeds to delete among the ManagedSeeds that are empty and not protected should be based on hints, perhaps again in the form of annotations, that could be added manually by the user, as well as other factors, see [Prioritizing ManagedSeed Deletion](#prioritizing-managedseed-deletion).

#### Prioritizing ManagedSeed Deletion

To help the controller decide which empty ManagedSeeds are to be deleted first, the user could manually annotate ManagedSeeds with a *seed priority annotation* such as `seedmanagement.gardener.cloud/priority`. ManagedSeeds with lower priority are more likely to be deleted first. If not specified, a certain default value is assumed, for example 3.

Besides this annotation, the controller should take into account also other factors, such as the current seed conditions (`NotReady` should be preferred for deletion over `Ready`), as well as its age (older should be preferred for deletion over newer). 

## Auto-Scaling Seeds

The most interesting and advanced automated seed management feature is making sure that a Garden cluster has enough seeds registered to schedule new shoots (and, in the future, reschedule shoots from drained seeds) without exceeding the seeds capacity for shoots, but not more than actually needed at any given moment. This would involve introducing an auto-scaling mechanism for seeds in Garden clusters. 

The proposed solution builds upon the ideas introduced earlier. The [`ManagedSeedSet`](#managedseeds) resource (and in the future, also the `ManagedSeedDeployment` resource) could have a `scale` subresource that changes the `replicas` field. This would allow a new "seed autoscaler" controller to scale these resources via a special "autoscaler" resource (for example `SeedAutoscaler`), similarly to how the Kubernetes [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) controller scales pods, as described in [Horizontal Pod Autoscaler Walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/).

The primary metric used for scaling should be the number of shoots already scheduled onto that seed either as a direct value or as a percentage of the seed's capacity for shoots introduced in [Ensuring Seeds Capacity for Shoots Is Not Exceeded](#ensuring-seeds-capacity-for-shoots-is-not-exceeded) (*utilization*). Later, custom metrics based on other resources, including provider-specific resources, could be considered as well.

> **Note:** Even if the controller is called *Horizontal Pod Autoscaler*, it is capable of scaling any resource with a `scale` subresource, using any custom metric. Therefore, initially it was proposed to use this controller directly. However, a number of important drawbacks were identified with this approach, and so it is no longer proposed here.

### SeedAutoscaler Resource

The SeedAutoscaler automatically scales the number of [ManagedSeeds](#managedseeds) in a [ManagedSeedSet](#managedseedsets) based on observed resource utilization. The resource could be any resource that is tracked via the `capacity` and `allocatable` fields in the Seed status, including in particular the number of shoots already scheduled onto the seed.

The SeedAutoscaler is implemented as a custom resource and a new controller. The resource determines the behavior of the controller. The `SeedAutoscaler` resource has a `scaleTargetRef` that specifies the target resource to be scaled, the minimum and maximum number of replicas, as well as a list of metrics. The only supported metric type initially is `Resource` for resources that are tracked via the `capacity` and `allocatable` fields in the Seed status. The resource target can be of type `Utilization` or `AverageValue`.

Example `SeedAutoscaler` resource:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: SeedAutoscaler
metadata:
  name: crazy-botany
  namespace: garden
spec:
  scaleTargetRef:
    apiVersion: seedmanagement.gardener.cloud/v1alpha1
    kind: ManagedSeedSet
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

### SeedAutoscaler Controller

`SeedAutoscaler` resources are reconciled by a new *seed autoscaler controller*, either in `gardener-controller-manager` or out-of-tree, similar to [cluster-autoscaler](https://github.com/gardener/autoscaler). The controller periodically adjusts the number of replicas in a ManagedSeedSet to match the observed average resource utilization to the target specified by user.

> **Note:** The SeedAutoscaler controller should perhaps not be limited to evaluating only metrics, it could also take into account taints, label selectors, etc. This is not yet reflected in the example `SeedAutoscaler` resource above. Such details are intentionally not specified in this GEP, they should be further explored in the issues created to track the actual implementation.

#### Evaluating Metrics for Autoscaling

The metrics used by the controller, for example the `shoots` metric above, could be evaluated in one of the following ways:

* Directly, by looking at the `capacity` and `allocatable` fields in the Seed status and comparing to the actual resource consumption calculated by simply counting all shoots that meet a certain criteria (e.g. shoots that are scheduled onto the seed), then taking an average over all seeds in the set.
* By sampling existing metrics exported for example by [`gardener-metrics-exporter`](https://github.com/gardener/gardener-metrics-exporter). 

The second approach decouples the seed autoscaler controller from the actual metrics evaluation, and therefore allows plugging in new metrics more easily. It also has the advantage that the exported metrics could also be used for other purposes, e.g., for triggering Prometheus alerts or building Grafana dashboards. It has the disadvantage that the seed autoscaler controller would depend on the metrics exporter to do its job properly. 
