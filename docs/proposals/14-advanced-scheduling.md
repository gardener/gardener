# Advanced Scheduling

This proposal describes certain advanced scheduling features, such as:

* Scheduling shoots on seeds based on taints and tolerations.
* Migrating shoots away from unhealthy seeds.
* Descheduling and migrating shoots away from healthy but overloaded seeds.
 
These features are considered "advanced", because they could be helpful in certain situations, but are not considered essential for regular operations. Implementing them would involve changes to various existing Gardener components, as well as perhaps introducing new ones. This document describes these features in more detail and proposes a design approach for some of them.

In Gardener, scheduling shoots onto seeds is quite similar to scheduling pods onto nodes in Kubernetes. Therefore, a guiding principle behind the proposed design approaches is taking advantage of best practices and existing components already used in Kubernetes. 

Some of these features depend on [GEP-7: Shoot Control Plane Migration](07-shoot-control-plane-migration.md), including avoiding "split brain" situations using [leader election](07-shoot-control-plane-migration.md#leader-election-and-control-plane-termination).

**Note:** These features were initially proposed as part [GEP-13: Automated Seed Management](13-automated-seed-management.md), but were later moved into their own GEP. Although valid, they are considered lower priority than the features described in GEP-13. Therefore, this GEP is not expected to progress beyond draft state in the near future.

## Taints and Tolerations for Seeds and Shoots

This feature is very similar to using taints and tolerations for nodes and pods in Kubernetes, as described in [Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). *Taints* are applied to seeds, and allow a seed to repel a set of shoots. *Tolerations* are applied to shoots, and allow (but do not require) the shoots to schedule onto seeds with matching taints.

Taints and tolerations work together to ensure that shoots are not scheduled onto inappropriate seeds. One or more taints are applied to a seed; this marks that the seed should not accept any shoots that do not tolerate the taints.

This feature is a prerequisite for the remaining features proposed in this GEP, and is also valuable on its own, therefore it is described first.

Taints on seeds and tolerations on shoots are already partially implemented in Gardener. They consist of a key and an (optional) value that should match for a toleration to tolerate a taint. One such taint is `gardener.cloud/protected`, which is used to mark a seed as protected, which means that it may only be used by shoots in the `garden` namespace. Taints and tolerations are also taken into account by the scheduler when scheduling shoots onto seeds. In order to schedule a shoot onto a seed that has taints, all of the seed taints must be tolerated by the shoot.

This proposal adds to the existing implementation the concept of [taint effects](#taint-effects), as explained below.

### Taint Effects

In addition to a key and a value, a taint can have an *effect*. The possible taint effects are the same as in Kubernetes: `NoSchedule`, `PreferNoSchedule`, and `NoExecute`. Since multiple taints can be put on the same seed and multiple tolerations on the same shoot, only the taints that are not tolerated by a shoot have the indicated effects on it. In particular:

* If there is at least one un-tolerated taint with effect `NoSchedule` then Gardener will not schedule the shoot onto that seed. Since this is what the scheduler does already if an effect is not specified, `NoSchedule` is the implicit effect of a taint with no effects specified.
* If there is no un-tolerated taint with effect `NoSchedule` but there is at least one un-tolerated taint with effect `PreferNoSchedule` then Gardener will try to not schedule the shoot onto the seed.
* If there is at least one un-tolerated taint with effect `NoExecute` then the shoot will be evicted from the seed (if it is already running on the seed), and will not be scheduled onto the seed (if it is not yet running on the seed).
 
## Migrating Shoots Away from Unhealthy Seeds

There are situations in which a seed can't function normally and should be either deleted or repaired "offline". This could be made easier by migrating shoots away from unhealthy seeds to healthy ones automatically or semi-automatically. This scenario is triggered by detecting an unhealthy seed. Once such a seed is detected, it should be drained of all its shoots, which means that they should be evicted from the seed and scheduled to other, healthy seeds.

### Marking Seeds as "Unschedulable"

The first step in migrating shoots away from an unhealthy seed is marking the seed as being "unschedulable" via a special taint, for example `seed.gardener.cloud/unschedulable`, with a `NoSchedule` effect. Initially, this could simply be done manually, or perhaps assisted by a special `gardenctl` command, for example `gardenctl taint <seed> `. In addition, the `gardenctl drain <seed>` command described below should add the taint to the specified seed prior to actually evicting all currently scheduled shoots.

Marking a seed as "unschedulable" prevents any new shoots to be scheduled onto this seed, due to the `NoSchedule` effect of the taint. It is useful for evicting shoots away from unhealthy seeds, but not limited to this use case. Operators might want to mark some seeds as "unschedulable" also in cases where a seed temporarily cannot accept more shoots.

Later, detecting unhealthy seeds and marking them as "unschedulable" could also be done automatically. This could be achieved by extending the `gardenlet` seed controller. In addition to setting certain seed conditions, the seed controller could also add the relevant taints with `NoSchedule` effects. Such taints could include for example:

* `seed.gardener.cloud/not-ready`: The seed is `NotReady`. 
* `seed.gardener.cloud/over-capacity`: The seed's capacity for shoots has been exceeded, see [Ensuring Seeds Capacity for Shoots Is Not Exceeded](13-automated-seed-management.md#ensuring-seeds-capacity-for-shoots-is-not-exceeded).

**Note:** The above taints are only meant as examples. In particular, the `seed.gardener.cloud/not-ready` taint could be valuable if in some cases shoots could be asked to tolerate a "not-ready" taint, similarly to pods tolerating not-ready nodes. If this is not the case, then such a taint should probably not be introduced. 

### Evicting Shoots from Drained Seeds

The next step in migrating shoots away from an unhealthy seed is actually draining the seed, which involves evicting all shoots from this seed. This could be achieved in one of the following ways:

* By manually marking all seed shoots as orphaned, by deleting the `seedName` field in the shoot specification.
* By a special `gardenctl` command, for example `gardenctl drain <seed>`, that would automate the action described above. It should also add the `seed.gardener.cloud/unschedulable` taint to the specified seed.

**Note:** The behavior described here is similar to the behavior of the `kubectl drain <node>` command. This command adds a taint with a `NoSchedule` effect to the node, but doesn't add any taints with a `NoExecute` effect. Instead, it simply forcibly evicts all pods from the node.

### Rescheduling Shoots to Healthy Seeds 

The next step in migrating shoots away from an unhealthy seed is rescheduling all orphaned shoots onto healthy seeds by `gardener-scheduler`. The scheduler reschedules shoots for which the `seedName` field has been unset, while also taking into accounts taints and tolerations, as well as the seed capacity for shoots, as described in [Taints and Tolerations for Seeds and Shoots](#taints-and-tolerations-for-seeds-and-shoots) and [Ensuring Seeds Capacity for Shoots Is Not Exceeded](13-automated-seed-management.md#ensuring-seeds-capacity-for-shoots-is-not-exceeded).

To reschedule a shoot, the scheduler updates the `seedName` field in the shoot specification with the name of the new seed. 

### Migrating the Shoots to the New Seeds

The final step is actually migrating the shoots control planes from the old to the new seeds. The migration is triggered by the updates to the `seedName` field in the shoot specification by the scheduler described above.

The migration process is described in more detail in [GEP-7: Shoot Control Plane Migration](07-shoot-control-plane-migration.md). 

## Descheduling Shoots from Healthy but Overloaded Seeds

In certain situations, for example if the seed capacity for shoots drops below its initial value for whatever reason, it might be required to deschedule shoots from an otherwise healthy but already overloaded seed. In this case, only a certain number of shoots should be descheduled from the overloaded seed and scheduled onto other seeds. 

This feature is considered lower priority than the features described above, and is therefore not described here in more detail yet. More concrete design ideas could be collected by looking into [Descheduler for Kubernetes](https://github.com/kubernetes-sigs/descheduler), a component that deschedules pods from Kubernetes nodes.
