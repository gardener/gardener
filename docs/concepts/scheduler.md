---
title: Gardener Scheduler
description: Understand the configuration and flow of the controller that assigns a seed cluster to newly created shoots
---

## Overview

The Gardener Scheduler is in essence a controller that watches newly created shoots and assigns a seed cluster to them.
Conceptually, the task of the Gardener Scheduler is very similar to the task of the Kubernetes Scheduler: finding a seed for a shoot instead of a node for a pod.

Either the scheduling strategy or the shoot cluster purpose hereby determines how the scheduler is operating.
The following sections explain the configuration and flow in greater detail.

## Why Is the Gardener Scheduler Needed?

### 1. Decoupling

Previously, an admission plugin in the Gardener API server conducted the scheduling decisions.
This implies changes to the API server whenever adjustments of the scheduling are needed.
Decoupling the API server and the scheduler comes with greater flexibility to develop these components independently.

### 2. Extensibility

It should be possible to easily extend and tweak the scheduler in the future.
Possibly, similar to the Kubernetes scheduler, hooks could be provided which influence the scheduling decisions.
It should be also possible to completely replace the standard Gardener Scheduler with a custom implementation.

## Algorithm Overview

The following **sequence** describes the steps involved to determine a seed candidate:

1. Determine usable seeds with "usable" defined as follows:
   * no `.metadata.deletionTimestamp`
   * `.spec.settings.scheduling.visible` is `true`
   * `.status.lastOperation` is not `nil`
   * conditions `GardenletReady`, `BackupBucketsReady` (if available) are `true`
1. Filter seeds:
   * matching `.spec.seedSelector` in `CloudProfile` used by the `Shoot`
   * matching `.spec.seedSelector` in `Shoot`
   * having no network intersection with the `Shoot`'s networks (due to the VPN connectivity between seeds and shoots their networks must be disjoint)
   * whose taints (`.spec.taints`) are tolerated by the `Shoot` (`.spec.tolerations`)
   * whose capacity for shoots would not be exceeded if the shoot is scheduled onto the seed, see [Ensuring seeds capacity for shoots is not exceeded](#ensuring-seeds-capacity-for-shoots-is-not-exceeded)
   * which have at least three zones in `.spec.provider.zones` if shoot requests a high available control plane with failure tolerance type `zone`.
1. Apply active [strategy](#strategies) e.g., _Minimal Distance strategy_
1. Choose least utilized seed, i.e., the one with the least number of shoot control planes, will be the winner and written to the `.spec.seedName` field of the `Shoot`.

In order to put the scheduling decision into effect, the scheduler sends an update request for the `Shoot` resource to
the API server. After validation, the `gardener-apiserver` updates the `Shoot` to have the `spec.seedName` field set.
Subsequently, the `gardenlet` picks up and starts to create the cluster on the specified seed.

## Configuration

The Gardener Scheduler configuration has to be supplied on startup. It is a mandatory and also the only available flag.
[This yaml file](../../example/20-componentconfig-gardener-scheduler.yaml) holds an example scheduler configuration.

Most of the configuration options are the same as in the Gardener Controller Manager (leader election, client connection, ...).
However, the Gardener Scheduler on the other hand does not need a TLS configuration, because there are currently no webhooks configurable.

## Strategies

The scheduling strategy is defined in the _**candidateDeterminationStrategy**_ of the scheduler's configuration and can have the possible values `SameRegion` and `MinimalDistance`.
The `SameRegion` strategy is the default strategy.

### Same Region strategy

The Gardener Scheduler reads the `spec.provider.type` and `.spec.region` fields from the `Shoot` resource.
It tries to find a seed that has the identical `.spec.provider.type` and `.spec.provider.region` fields set.
If it cannot find a suitable seed, it adds an event to the shoot stating that it is unschedulable.

### Minimal Distance strategy

The Gardener Scheduler tries to find a valid seed with minimal distance to the shoot's intended region.
Distances are configured via `ConfigMap`(s), usually per cloud provider in a Gardener landscape.
The configuration is structured like this:
- It refers to one or multiple `CloudProfile`s via annotation `scheduling.gardener.cloud/cloudprofiles`.
- It contains the declaration as `region-config` via label `scheduling.gardener.cloud/purpose`.
- If a `CloudProfile` is referred by multiple `ConfigMap`s, only the first one is considered.
- The `data` fields configure actual distances, where _key_ relates to the `Shoot` region and _value_ contains distances to `Seed` regions.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <name>
  namespace: garden
  annotations:
    scheduling.gardener.cloud/cloudprofiles: cloudprofile-name-1{,optional-cloudprofile-name-2,...}
  labels:
    scheduling.gardener.cloud/purpose: region-config
data:
  region-1: |
    region-2: 10
    region-3: 20
    ...
  region-2: |
    region-1: 10
    region-3: 10
    ...
```

> Gardener provider extensions for public cloud providers usually have an example weight `ConfigMap` in their repositories.
> We suggest to check them out before defining your own data.

If a valid seed candidate cannot be found after consulting the distance configuration, the scheduler will fall back to
the Levenshtein distance to find the closest region. Therefore, the region name
is split into a base name and an orientation. Possible orientations are `north`, `south`, `east`, `west` and `central`.
The distance then is twice the Levenshtein distance of the region's base name plus a correction value based on the
orientation and the provider.

If the orientations of shoot and seed candidate match, the correction value is 0, if they differ it is 2 and if
either the seed's or the shoot's region does not have an orientation it is 1.
If the provider differs, the correction value is additionally incremented by 2.

Because of this, a matching region with a matching provider is always preferred.

### Special handling based on shoot cluster purpose

Every shoot cluster can have a purpose that describes what the cluster is used for, and also influences how the cluster is setup (see [Shoot Cluster Purpose](../usage/shoot-basics/shoot_purposes.md) for more information).

In case the shoot has the `testing` purpose, then the scheduler only reads the `.spec.provider.type` from the `Shoot` resource and tries to find a `Seed` that has the identical `.spec.provider.type`.
The region does not matter, i.e., `testing` shoots may also be scheduled on a seed in a complete different region if it is better for balancing the whole Gardener system.

## `shoots/binding` Subresource

The `shoots/binding` subresource is used to bind a `Shoot` to a `Seed`. On creation of a shoot cluster/s, the scheduler updates the binding automatically if an appropriate seed cluster is available.
Only an operator with the necessary RBAC can update this binding manually. This can be done by changing the `.spec.seedName` of the shoot. However, if a different seed is already assigned to the shoot, this will trigger a control-plane migration. For required steps, please see [Triggering the Migration](../operations/control_plane_migration.md#triggering-the-migration).

## `spec.schedulerName` Field in the `Shoot` Specification

Similar to the `spec.schedulerName` field in `Pod`s, the `Shoot` specification has an optional `.spec.schedulerName` field. If this field is set on creation, only the scheduler which relates to the configured name is responsible for scheduling the shoot.
The `default-scheduler` name is reserved for the default scheduler of Gardener.
Affected Shoots will remain in `Pending` state if the mentioned scheduler is not present in the landscape.

## `spec.seedName` Field in the `Shoot` Specification

Similar to the `.spec.nodeName` field in `Pod`s, the `Shoot` specification has an optional `.spec.seedName` field. If this field is set on creation, the shoot will be scheduled to this seed. However, this field can only be set by users having RBAC for the `shoots/binding` subresource. If this field is not set, the `scheduler` will assign a suitable seed automatically and populate this field with the seed name.

## `seedSelector` Field in the `Shoot` Specification

Similar to the `.spec.nodeSelector` field in `Pod`s, the `Shoot` specification has an optional `.spec.seedSelector` field.
It allows the user to provide a label selector that must match the labels of the `Seed`s in order to be scheduled to one of them.
The labels on the `Seed`s are usually controlled by Gardener administrators/operators - end users cannot add arbitrary labels themselves.
If provided, the Gardener Scheduler will only consider as "suitable" those seeds whose labels match those provided in the `.spec.seedSelector` of the `Shoot`.

By default, only seeds with the same provider as the shoot are selected. By adding a `providerTypes` field to the `seedSelector`,
a dedicated set of possible providers (`*` means all provider types) can be selected.

## Ensuring a Seed's Capacity for Shoots Is Not Exceeded

Seeds have a practical limit of how many shoots they can accommodate. Exceeding this limit is undesirable, as the system performance will be noticeably impacted. Therefore, the scheduler ensures that a seed's capacity for shoots is not exceeded by taking into account a maximum number of shoots that can be scheduled onto a seed.

This mechanism works as follows:

* The `gardenlet` is configured with certain *resources* and their total *capacity* (and, for certain resources, the amount *reserved* for Gardener), see [/example/20-componentconfig-gardenlet.yaml](../../example/20-componentconfig-gardenlet.yaml). Currently, the only such resource is the maximum number of shoots that can be scheduled onto a seed.
* The `gardenlet` seed controller updates the `capacity` and `allocatable` fields in the Seed status with the capacity of each resource and how much of it is actually available to be consumed by shoots. The `allocatable` value of a resource is equal to `capacity` minus `reserved`.
* When scheduling shoots, the scheduler filters out all candidate seeds whose allocatable capacity for shoots would be exceeded if the shoot is scheduled onto the seed.

## Failure to Determine a Suitable Seed

In case the scheduler fails to find a suitable seed, the operation is being retried with exponential backoff.
The reason for the failure will be reported in the `Shoot`'s `.status.lastOperation` field as well as a Kubernetes event (which can be retrieved via `kubectl -n <namespace> describe shoot <shoot-name>`).

## Current Limitation / Future Plans

- Azure unfortunately has a geographically non-hierarchical naming pattern and does not start with the continent. This is the reason why we will exchange the implementation of the `MinimalDistance` strategy with a more suitable one in the future.
