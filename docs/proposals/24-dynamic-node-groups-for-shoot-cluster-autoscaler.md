---
title: Dynamic Node Groups for Shoot Cluster Autoscaler
gep-number: 23
creation-date: 2023-09-30
status: implementable
authors:
- "@elankath"
reviewers:
- "@main-reviewer-1"
- "@main-reviewer-2"
---

# GEP-24: Moving to Dynamic Node Groups for Shoot Cluster Autoscaler

## Table of Contents

- [GEP-24: Moving to Dynamic Node Groups for Shoot Cluster Autoscaler](#gep-24-moving-to-dynamic-node-groups-for-shoot-cluster-autoscaler)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Existing Solution](#existing-solution)
      - [Worker Actuator Reconciliation](#worker-actuator-reconciliation)
      - [MachineDeployment Creation Details](#machinedeployment-creation-details)
        - [MachineDeployment Min/Max Distribution](#machinedeployment-minmax-distribution)
          - [Pool-0:1:1:1:2 (min:max:maxsurge:maxunavail:numzones)](#pool-01112-minmaxmaxsurgemaxunavailnumzones)
          - [Pool-3:5:1:1:2 (min:max:maxsurge:maxunavail:numzones)](#pool-35112-minmaxmaxsurgemaxunavailnumzones)
    - [Deficiencies of Existing Solution](#deficiencies-of-existing-solution)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [0. Add PoolSize Validation / Fix Existing Clusters](#0-add-poolsize-validation--fix-existing-clusters)
    - [1. Extend MCM MachineDeployment](#1-extend-mcm-machinedeployment)
    - [1.1 Remove Minimum/Maximum Static Computation](#11-remove-minimummaximum-static-computation)
    - [2. Node Group Auto Discovery](#2-node-group-auto-discovery)
    - [3. Dynamic Node Group Sizing.](#3-dynamic-node-group-sizing)
    - [3.1 Lax-Greedy Sizing](#31-lax-greedy-sizing)
      - [Pros/Cons](#proscons)
      - [Examples](#examples)
        - [Pool-3:4:2:2:3 (min:max:maxsurge:maxunavail:numzones)](#pool-34223-minmaxmaxsurgemaxunavailnumzones)
          - [Scan-0](#scan-0)
          - [Scan-1](#scan-1)
          - [Scan-2](#scan-2)
          - [Scan-3](#scan-3)
    - [3.2 Backward-Compatible](#32-backward-compatible)
      - [Pros/Cons](#proscons-1)
      - [Examples](#examples-1)
        - [Pool-3:4:2:2:3 (min:max:maxsurge:maxunavail:numzones)](#pool-34223-minmaxmaxsurgemaxunavailnumzones-1)
          - [Scan-0](#scan-0-1)
          - [Scan-1](#scan-1-1)
          - [Scan-2](#scan-2-1)
          - [Pool-0:2:1:1:2 (min:max:maxsurge:maxunavail:numzones)](#pool-02112-minmaxmaxsurgemaxunavailnumzones)
          - [Scan-0](#scan-0-2)
          - [Scan-1](#scan-1-2)
    - [4. Shoot Spec Enhancement](#4-shoot-spec-enhancement)
    - [TODO](#todo)

## Summary

Presently, the `NodeGroups` for the shoot `cluster-autoscaler`  are given static `Minimum` and `Maximum` sizes computed in the gardener extension providers during shoot reconciliation. This has several disadvantages since the static computation preferences zones that are specified earlier over zones specified later. In addition, for cases where the number of zones exceeds the `Max` specified for the worker pool, the node-group is assigned a zero `Max` size.

This document outlines deficiences in the existing solution and proposes moving to the CA's node group auto-discovery mechanism where node groups can be discovered and sized at runtime. 

## Motivation

TODO: Add Sequence Diagram for the below.

Let us first study how `MachineDeployments` are currently generated and how node groups are currently computed for the `cluster-autoscaler`.


### Existing Solution

#### Worker Actuator Reconciliation 

The Worker [generic.Actuator.Reconcile](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/extensions/pkg/controller/worker/genericactuator/actuator_reconcile.go#L42) implementation delegates to [genericactuator.GenerateMachineDeployments](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/extensions/pkg/controller/worker/genericactuator/interface.go#L46) to generate [worker.MachineDeployments](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/extensions/pkg/controller/worker/machines.go#L39) for a shoot clulster during shoot reconciliation.

These Worker MachineDeployments's are converted into MCM [MachineDeployments](https://pkg.go.dev/github.com/gardener/machine-controller-manager@v0.50.0/pkg/apis/machine/v1alpha1#MachineDeployment)'s and then deployed into the shoot control plane.

These worker deployments are also set on the [Worker Status](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/extensions/pkg/controller/worker/genericactuator/actuator_reconcile.go#L470).

The [Cluster Autoscaler Deployer](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/pkg/component/clusterautoscaler/cluster_autoscaler.go#L103) then constructs the deployment command string via [computeCommand](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/pkg/component/clusterautoscaler/cluster_autoscaler.go#L397) to iterate through these worker deployments and create static node groups as follows:

```go
for _, machineDeployment := range c.machineDeployments {
  command = append(command, fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment.Minimum, machineDeployment.Maximum, c.namespace, machineDeployment.Name))
}
```
This is the format `--nodes=nodeGroupMin:nodeGroupMax:nodeGroupName`
Effectively we currently have a `1:1` correspondence between a `MachineDeployment` and a CA `NodeGroup`. The `NodeGroup` is the abstraction used by the CA to represent a set of nodes that have the same capacity and set of labels and within which scale-up and scale-down operations can be performed.


#### MachineDeployment Creation Details

Most garden extension providers have a helper method [generateMachineConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/c3439f0a7e6bd76dda464be529396e0590983c55/pkg/controller/worker/machines.go#L87) that iterates over the worker pools and the pool zones and generates the [worker.MachineDeployment](https://github.com/gardener/gardener/blob/da46645d62e8487c7000d72208afcde8c293fc70/extensions/pkg/controller/worker/machines.go#L39)'s for each worker pool and zone combination as can be seen from the snippet below.

```go
for _, pool := range w.worker.Spec.Pools {
  zoneLen := int32(len(pool.Zones))
deploymentName = fmt.Sprintf("%s-%s-z%d", w.worker.Namespace, pool.Name, zoneIndex+1)
  for zoneIndex, zone := range pool.Zones {
    //...
    machineDeployments = append(machineDeployments, worker.MachineDeployment{
      //...
      Name:                 deploymentName,
      Minimum:              worker.DistributeOverZones(zoneIdx, pool.Minimum, zoneLen),
      Maximum:              worker.DistributeOverZones(zoneIdx, pool.Maximum, zoneLen),
      MaxSurge:             worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum),
      MaxUnavailable:       worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum),
      Labels:               addTopologyLabel(pool.Labels, zone),
      Annotations:          pool.Annotations,
      //...
    })
  }
}
```

##### MachineDeployment Min/Max Distribution 

As can be seen from the snippet above, the machine deployment `Minimum` and `Maximum` delegates to the helper method `DistributeOverZones` which is coded as below:

```go
func DistributeOverZones(zoneIndex, size, zoneSize int32) int32 {
	first := size / zoneSize
	second := int32(0)
	if zoneIndex < (size % zoneSize) {
		second = 1
	}
	return first + second
}
```
Even machine deployment `MaxSurge` and `MaxUnavailable` delegates to `DistributePositiveIntOrPercent` which in turn delegates to `DistributeOverZones`

The above computation has issues as can be seen below for generated machine deployments.

###### Pool-0:1:1:1:2 (min:max:maxsurge:maxunavail:numzones)
```
MachineDeployment_Z0(Minimum=0|Maximum:1|MaxSurge:1|MaxUnavailable:1)
MachineDeployment_Z1(Minimum=0|Maximum:0|MaxSurge:0|MaxUnavailable:0)
```
Consider the case above where `numzones:2` > worker pool `max:1`, We can see that the machinedeployment for `Z1` is useless as its `Min`, `Max`, `MaxSurge` and `MaxUnavailable` are all zero making the corresponding CA `NodeGroup` also useless.


###### Pool-3:5:1:1:2 (min:max:maxsurge:maxunavail:numzones)
```
MachineDeployment_Z0(Minimum=2|Maximum:3|MaxSurge:1|MaxUnavailable:1)
MachineDeployment_Z1(Minimum=1|Maximum:2|MaxSurge:0|MaxUnavailable:0)
```
Consider the case above - `MaxSurge`  for `Z1` is computed as `0`

The above cases are not un-common. Thre are `~65` Worker Pools in LIVE Landscape with `0 < WorkerPoolMax < WorkerPoolZones`




### Deficiencies of Existing Solution

1. The current static node group distribution skews distribution towards earlier specified zones over later specified zones.
2. If zone length exceeds worker pool max, then later node groups get zero values effectively becoming useless.
3. If the earlier node groups have quota issues, then later node groups have reduced capacity and cannot meet the specified availability. 


### Goals

1. We remove the current static node group assignment and move to a dynamic node group policy leveraging [node-group-auto-discovery](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/main.go#L156)
2. We change our MCM [CloudProvider](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/mcm/mcm_cloud_provider.go#L52) implementaion to support dynamic node groups and node group auto-discovery.
3. We change the deployment of the CA Deployer to construct the command string differently.
4. We fix invalid shoot clusters on all landscapes where `WorkerPool.Maxium < WorkerPool.NumZones` 
   1. We also introduce validation for new shoot clusters to ensure this constraint is honored.

### Non-Goals

1. No revamp of core `cluster-autoscaler` mechanics is considered in this proposal.

## Proposal

### 0. Add PoolSize Validation / Fix Existing Clusters 

As was described earlier, our current distribution logic for worker pools can result in creating non-sensical NodeGroups where the `MaxSize` of the NodeGroup is computed as `0`. Since we will preserve the current distribution logic in our `Backward-Compatible` NodeGroup sizing strateg, we should add a validation check to reject shoot specs where `WorkerPool.Maximum < WorkerPool.NumZones`. We should also fix existing shoot specs.

### 1. Extend MCM MachineDeployment

Unlike the [worker MachineDeployment](https://github.com/gardener/gardener/blob/a3632ea5315d104d0ed3dd47e6d17f53cfe0e877/extensions/pkg/controller/worker/machines.go#L39), the [MCM MachineDeployment](https://pkg.go.dev/github.com/gardener/machine-controller-manager@v0.50.0/pkg/apis/machine/v1alpha1#MachineDeployment) as of today does not preserve the statically computed `Minimum` and `Maximum`.  It only possesses a `Replicas` field. We propose extending [MachineDeploymentSpec](https://github.com/gardener/machine-controller-manager/blob/d0fdc315087158d41f31d0c4bbbb25af9845eb0f/pkg/apis/machine/types.go#L412) with a `PoolMinimum` and `PoolMaximum`, making sure these are populated in the garden extension provider and computing `NodeGroup` `Minimum` and `Maximum` at runtime in the CA. These would be set as usual during the MCD generation in the garden extension provider.

### 1.1 Remove Minimum/Maximum Static Computation

The existing code in the garden extension provider which statically computes Worker MachineDeployment Minimum/Maximum can be removed. If we are using a distribution based strategy, we can always do this at runtime in the CA.

### 2. Node Group Auto Discovery

We do not perform a strict, static mapping of CA Node Groups limits to machine deployment limits.  We may continue to generate the `MachineDeployments` as they are done currently for ease of backward compatability.

Instead of using current statically specified [--nodes](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/externalgrpc/examples/external-grpc-cloud-provider-service/main.go#L69) flag, we will dynamically compute and discover our node groups using the [--node-group-auto-discovery](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/externalgrpc/examples/external-grpc-cloud-provider-service/main.go#L72) to the CA.

This flag is expressed as a value of the form. `<name of discoverer>:[<key>[=<value>]]`. 
The key, value pairs should be used to look up the node groups.

The MCM forks the CA and injects its own `CloudProvider` implementation. This is present at this [location](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/mcm/mcm_cloud_provider.go).

For MCM CA `CloudProvider`, we propose simply using an `--node-group-auto-discovery` flag of the form `mcm:k8s.io/cluster-autoscaler/<shoot-cluster-name>`.

The CA encapsulates the above in [cloudprovider.NodeGroupDiscoveryOptions.NodeGroupAutoDiscoverySpecs](https://github.com/kubernetes/autoscaler/blob/53ca6b941b16e205418b05583b809415897f3da7/cluster-autoscaler/cloudprovider/node_group_discovery_options.go#L24) and passes this when [building](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/builder/builder_mcm.go#L36) the MCM CloudProvider


The above will permit the MCM CloudProvider to discover all the Machine Deployments for the shoot cluster.

### 3. Dynamic Node Group Sizing.

The [cloudprovider.CloudProvider](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/cloud_provider.go#L100) interface is the facade between the CA and the cloud platform.

[cloudprovider.NodeGroup](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/cloud_provider.go#L163) is the CA abstraction used to represent the set of cloud-provider nodes which are in-turn represented by [cloudprovider.Instances](https://github.com/gardener/autoscaler/blob/053c0d5176cb2d195e3baf333b05ceea99eedb58/cluster-autoscaler/cloudprovider/cloud_provider.go#L238)

The CA carries out scale-down and scale-up operations within a `NodeGroups` respecting the constraints given by the `NodeGroup.MinSize()` and `NodeGroup.MaxSize()` methods.

The CA runs a reconcile loop every `scanPeriod` interval (default: `10s`). At the beginning of the reconcile loop, the CA invokes `CloudProvider.Refresh()` to permit the cloud provider implementation to update its cache and then issues a call to `CloudProvider.NodeGroups()` to retrieve the latest `[]NodeGroup`.  The MCM CloudProvider implements this by interrogating the machine deployments.
Each `NodeGroup` continues to be associated with its corresponding `MachineDeployment` for the zone.

We will offer 2 strategies for node-group sizing: *Lax Greedy Sizing* and *Equitable Sizing*

### 3.1 Lax-Greedy Sizing
1. If this strategy is selected, then for each `NodeGroup` the `NodeGroup.MaxSize()` is initially returned as the `MachineDeployment.Spec.PoolMaximum` and the `NodeGroup.MinSize()` is given as the `0` if there are no nodes provisioned for the shoot cluster.
1. The CA will execute scale-up/scale-down activities for the node groups if needed. If a NodeGroup does not have quota it will back-off and try another NodeGroup.
1. For the next scan, we compute `NodeGroup.MaxSize()` as follows
     1. `NodeGroup.MaxSize() = PoolMaximum-CountOfAllNodesMaterializedInOtherNodeGroups)`


#### Pros/Cons
1. Pro: This strategy has the primary advantage that workload can be provisioned within the zone for the NodeGroup. It can be considered as work-load friendly
2. Pro: It can handle back-off well. If nodes can't be provisioned in one node group, the other node group can take up the slack.
3. Con: All instances may stay in one availability zone even if multiple are configured. While this may be advantageous short-term, it may impact availability if the zone goes down.
4. Con: We need to add support for scale from zero for extended/ephmeral resources. We have [Gardener CA #132](https://github.com/gardener/autoscaler/issues/132) for this.

#### Examples

##### Pool-3:4:2:2:3 (min:max:maxsurge:maxunavail:numzones)

* PoolMax:4

###### Scan-0
```
NG-0: Min:0, Max:4, Launched: 0
NG-1: Min:0, Max:4, Launched: 0
NG-2: Min:0, Max:4, Launched: 0
```

###### Scan-1

```
NG-0: Min:0, Max:4, Launched: 1
NG-1: Min:0, Max:3, Launched: 0
NG-2: Min:0, Max:3, Launched: 0
```

###### Scan-2

```
NG-0: Min:0, Max:3, Launched: 2
NG-1: Min:0, Max:2, Launched: 1
NG-2: Min:0, Max:1, Launched: 0
```

###### Scan-3

```
NG-0: Min:0, Max:2, Launched: 2
NG-1: Min:0, Max:1, Launched: 1
NG-2: Min:0, Max:1, Launched: 1
```


### 3.2 Backward-Compatible
1. The backward compatible sizing strategy honours our current distrubition logic as much as possible.
1. Effectively for each `NodeGroup`, we compute the `NodeGroup.MaxSize` and `NodeGroup.MinSize` using the existing `DistributeOverZones` function 
    1. There is only one difference in that the maximum will not be permitted to be zero for any NodeGroup. This will be handled by validation and migration of existing clusters that break this rule (`PoolMaximum<numZones`).
    1. `NodeGroup.MaxSize()=DistributeOverZones(zoneIdx,machineDeployment.Spec.PoolMaximum,numZones)`
    1. `NodeGroup.MaxSize()=DistributeOverZones(zoneIdx,machineDeployment.Spec.PoolMinimum,numZones)`
1. The CA will execute scale-up/scale-down activities for the node groups if needed. If a NodeGroup does not have quota it will back-off and try another NodeGroup.

#### Pros/Cons
1. Pro: This strategy has the primary advantage that we remain backward-compatible with our current distribution logic. However, it does not handle 
2. Con: Unfortunately, it doesn't handle back-off well.

#### Examples


##### Pool-3:4:2:2:3 (min:max:maxsurge:maxunavail:numzones)

###### Scan-0
```
NG-0: Min:1, Max:2, Launched: 0
NG-1: Min:1, Max:1, Launched: 0
NG-2: Min:1, Max:1, Launched: 0
```

###### Scan-1

```
NG-0: Min:1, Max:2, Launched: 1
NG-1: Min:1, Max:1, Launched: 1
NG-2: Min:1, Max:1, Launched: 0
```

###### Scan-2

```
NG-0: Min:1, Max:2, Launched: 2
NG-1: Min:1, Max:1, Launched: 1
NG-2: Min:1, Max:1, Launched: 1
```

###### Pool-0:2:1:1:2 (min:max:maxsurge:maxunavail:numzones)

* PoolMax=2
* Let us take the case where `NG-0` is out of quota

###### Scan-0
```
NG-0: Min:0, Max:1, Launched: 0
NG-1: Min:0, Max:1, Launched: 0
```

###### Scan-1
```
NG-0: Min:0, Max:1, Launched: 0 (backoff)
NG-1: Min:0, Max:1, Launched: 1
```
* No further progress since `NG-1` has reached its computed `Max` and no nodes can be provisioned in `NG-0`.  This is our current behaviour unfortunately.

### 4. Shoot Spec Enhancement

We add a `nodeGroupSizingStrategy` in our the `clusterAutoscaler` section of our `shoot` YAML

```yaml
clusterAutoscaler:
  sizing: Lax-Greedy|Backward-Compatible
```

### TODO
1. Discuss `MaxUnavailable` and `MaxSurge` handling. Should this be copied or distributed ?
2. Do we need further `NodeGroup` sizing strategies ?