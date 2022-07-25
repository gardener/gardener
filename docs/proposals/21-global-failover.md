---
title: Global Failover
gep-number: 21
creation-date: 2022-07-12
status: implementable
authors:
- "@jia-jerry"
- "@EmoinLanyu"
reviewers:
- "@main-reviewer-1"
- "@main-reviewer-2"
---

# GEP-21: Fail Over Multi-AZ Services by Isolating Unhealthy AZ

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
    - [New Changes](#new-changes)
    - [Control Failover / Failback Process](#control-failover--failback-process)
    - [Failover Workflow](#failover-workflow)
    - [Failback Workflow](#failback-workflow)
- [Future Plan](#future-plan)

## Summary

This `GEP` introduces an operating procedure allowing for Gardener operators to isolate an unhealthy AZ of a `Shoot` cluster, and consequently fail over services running inside to healthy AZs. This procedure consists of two steps: removing `MachineDeployment` in the unhealthy AZ, and applying `SubnetIsolation` to the unhealthy AZ. The `SubnetIsolation` is a new extension resource that is reconciled by the respective provider extension controller.

Moreover, it covers a failback procedure for services with multi-AZ setup once the unhealthy AZ is recovered.

## Motivation

Several outages in past years have shown that AZ-specific degradations / failures in hyperscaler services result in significant issues for services. There are multiple patterns of known issues.

1. Some services operate multiple instances of each components redundantly within a single AZ. This will likely provide uptimes approaching 99.5%, but such services are subject to AZ failures and remain offline until the underlying AZ is restored. We shouldn't fail over a `Shoot` cluster with single AZ HA services.
2. Often, multi-AZ HA services are able to stay available during AZ-specific degradations, such as slow disk IO, network lantency, etc. But their api performance may be downgraded tenfold.
3. In some edge cases, multi-AZ HA services may run into inconsistent state due to unexpected AZ degradations / failures. Manual operations are needed in order to recover the problematic service back to normal.

A global failover, i.e. not using an unhealthy AZ any longer, will bring benefits and reduce customer disruptions. For example, in case of pattern 2, by isolating the unhealthy AZ, service will be forced to failover, all traffic will be routed to healthy AZs, and consequently service performance will be back to normal quickly. In case of pattern 3, manual operations are inevitable for recovering services in inconsistent state, however forced failovers of other services can eliminate new inconsistent state during a long period of hyperscaler failures.

### Goals

Reduce customer disruptions during a long period of hyperscaler failure by isolating unhealthy AZ, enforcing a full zonal outage and triggering failovers / failbacks of services in a controlled way.

### Non-Goals

- Help services to achieve multi-AZ HA.
- Rebalance `Pod`s during / after failback.

## Proposal

### New Changes

#### New Annotation on `Shoot`

A new annotation is introduced: `shoot.gardener.cloud/isolated-zone`. The value is the ID of the AZ to be isolated. For example, for AWS, zone code may vary in different accounts. We should specify physical zone ID here.

#### Changing the `Shoot` Status

A new section in the `Shoot` status is added when failover / failback is triggered.

(TODO: how should we name the phases? Failover or ZoneIsolation?)

```yaml
status:
  zoneIsolation:
    isolatedZoneID: euc1-az1
    isolatedZoneCode: eu-central-1a
    phase: Failover # Failover | Failback
    status: Done # Pending | InProgress | Aborting | Done | Failed
    lastUpdateTime: 2022-07-17T00:00:00Z
```

#### New Gardener Admission Plugin

A new Gardener admission plugin is added. The new admission plugin blocks updating of `shoot.gardener.cloud/isolated-zone` annotation.

#### Changes to `Shoot` Control Flows

- Reconciliation flow:
  - Information about isolated AZ will be added to `Worker` during `deployWorker`.
  - New steps `deploySubnetIsolation` and `waitUntilSubnetIsolationReady` are added to Shoot reconciliation flow with `waitUntilWorkerReady` as dependency. `SubnetIsolation` resource will be deployed to the Seed cluster in `deploySubnetIsolation` step. The two steps will be skipped if `shoot.gardener.cloud/isolated-zone` annotation is not present.
  - New steps `destroyStaleSubnetIsolation` and `waitUntilStaleSubnetIsolationDeleted` are added to Shoot reconciliation flow. `waitUntilStaleSubnetIsolationDeleted` should be added as `deployInfrastructure`'s dependency. The two steps will be skipped if `shoot.gardener.cloud/isolated-zone` annotation is present.
- Maintenance flow:
  - Maintenance flow should be skipped if `shoot.gardener.cloud/isolated-zone` annotation is present.
- Deletion flow:
  - New steps `destroySubnetIsolation` and `waitUntilSubnetIsolationDeleted` are added to Shoot deletion flow.

#### Changes to `Worker` CRD

Add a new field to `Worker` spec and a new field to `Worker` status.

```yaml
spec:
  isolatedZoneID: euc1-az1
status:
  isolatedZoneCode: eu-central-1a
```

#### New CRD on `Seed` Cluster

A new CRD named `SubnetIsolation` is introduced. Different cloud providers handle network isolation differently. Each provider extension should maintain its own status.

When `SubnetIsolation` is created, the provider extension controller will carry out necessary actions to isolate the specified AZ of the `Shoot` cluster. On AWS, for example, a network ACL that blocks all traffic to and from the subnets in the unhealthy AZ will be created. The status / progress of the isolation will also be maintained in the `SubnetIsolation` resource.

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: SubnetIsolation
metadata:
  name: isolate-zone-a
  namespace: shoot--core--test-01
spec:
  isolatedZoneID: euc1-az1
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
  providerStatus:
    apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
    kind: SubnetIsolationStatus
    isolatedZoneCode: eu-central-1a
    aclID: acl-asdhfh2qiefjaskd
    aclStatus: created
```

### Control Failover / Failback Process

- To trigger the failover of a `Shoot`, add `shoot.gardener.cloud/isolated-zone` annotation to the `Shoot` with valid zone ID.
- To trigger the failback of a `Shoot` or to abort the failover of a `Shoot`, remove the `shoot.gardener.cloud/isolated-zone` annotation.
- Add `gardener.cloud/operation: reconcile` annotation to the `Shoot` to trigger reconciliation when failover / failback is needed.

### Failover Workflow

#### Update `Worker` to Remove `MachineDeployment`s in Isolated AZ

Information about zone isolation will be added to `Worker` resources. Provider extension controller will react to this change and rule out the unhealthy AZ when generating `MachineDeployment`s.
The final result would be that no `MachineDeployment` is created in the unhealthy AZ. And all current `MachineDeployment`s (as well as `Machine`s) in that AZ will be destroyed.

This way, we can ensure that the `Shoot` be in a healthy state, and we can avoid Machine Controller Manager to constantly recreate `Machine`s in the isolated unhealthy AZ.

Meanwhile, by shutting down the nodes in the unhealthy AZ and evicting pods on them, multi-AZ HA services get a chance to gracefully terminate the unhealthy instances.

#### Create `SubnetIsolation` Resource in `Seed` Cluster

New steps are added to `Shoot` reconciliation flow for the `Shoot` controllers of `gardenlet`. After `waitUntilWorkerReady` step is finished, `SubnetIsolation` resource will be created
in the `Seed` cluster (new steps: `deploySubnetIsolation` and `waitUntilSubnetIsolationReady`).

When `SubnetIsolation` controller in `Seed` cluster notices that a new `SubnetIsolation` is created in a `Shoot`'s namespace, it first waits (with timeout) for Machine Controller Manager to delete the `MachineDeployment`s in the unhealthy AZ. After that, the `SubnetIsolation` controller continues to do necessary actions - e.g. creating ACL for the `Shoot` in the unhealthy AZ.

### Failback Workflow

#### Update `Worker` to Recreate `MachineDeployment`s in Isolated AZ

Information about AZ isolation will be removed from `Worker` resources. Provider extension controller will react to this change and add back the isolated AZ when generating `MachineDeployment`s.

#### Delete `SubnetIsolation` Resource in `Seed` Cluster

New steps are added to `Shoot` reconciliation flow for the `Shoot` controllers of `gardenlet`. Before `deployInfrastructure`, `SubnetIsolation` will be deleted from `Seed` cluster. And when `SubnetIsolation` controller in `Seed` cluster notices that the `SubnetIsolation` is deleted in a `Shoot`'s namespace, it will do necessary actions - e.g. deleting previously created ACL of the `Shoot`.

## Future Plan

As of today, we lack a good indicator when to perform a global failover, except that we receive confirmations that hyperscaler has an unhealthy AZ. In the long run, global failure detection, which ensure proper state visibility, is required. The status discovery needs to happen on infrastructure level close to all workloads in order to be able to detect the extent of local issues within an AZ.
