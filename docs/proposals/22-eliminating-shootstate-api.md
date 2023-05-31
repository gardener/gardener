---
title: Eliminating the `ShootState` API
gep-number: 22
creation-date: 2023-06-01
status: implementable
authors:
- "@rfranzke"
reviewers:
- "@timebertt"
---

# GEP-22: Eliminating the `ShootState` API

## Table of Contents

<!-- TOC -->
- [GEP-22: Eliminating the `ShootState` API](#gep-22-eliminating-the-shootstate-api)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
  - [Alternatives](#alternatives)
  - [Future Improvements](#future-improvements)
<!-- TOC -->

## Summary

For each `Shoot` resource, `gardenlet` creates a corresponding `ShootState` resource which has the same lifecycle of the `Shoot`.
Today, such resources are used for two purposes:

1. Exposing the client CA certificate and key of the shoot cluster such that the `gardener-apiserver` is able to issue short-lived client certificates via the [`shoots/adminkubeconfig` subresource](../usage/shoot_access.md#shootsadminkubeconfig-subresource).
2. Storing the most crucial data for shoot clusters which is necessary to migrate and restore a cluster from one `Seed` to another (certificates, keys, secrets, relevant extensions state for created infrastructure or worker machines).

Especially due to the second purpose, the `ShootState` API imposes several scalability concerns and limitations for large Gardener installations.
Hence, this GEP proposes the elimination of said `ShootState` API by replacing it with lightweight alternatives.

This GEP focuses on the second use-case only and ignores the first use-case.
An alternative for it is worked on separately in [Introduce `core.gardener.cloud/v1beta1.InternalSecret` API #7999](https://github.com/gardener/gardener/issues/7999).

## Motivation

The `gardenlet` runs [two controllers](../concepts/gardenlet.md#shootstate-controller) for maintaining the most crucial data of shoot clusters in their `ShootState` resources:

- The [`shootstate-secret`](../../pkg/gardenlet/controller/shootstate/secret) controller watches `Secret`s labeled with `persist=true` in the shoot namespaces in the seed cluster. It persists all labels and data of such `Secret`s into the related `ShootState` resource.

In reality, those `Secret`s only change when the end-user initiates a credentials rotation operation, so rather rarely.
Still, duplicating all labels and data of such `Secret`s into the garden cluster increases the size of `ShootState` resources.

- The [`shootstate-extensions`](../../pkg/gardenlet/controller/shootstate/extensions) controller watches all resources of the `extensions.gardener.cloud/v1alpha1` API group in the shoot namespaces in the seed cluster. It persists all extensions state and resources in the respective `.status.state` and `.status.resources` fields. See also [this document](../extensions/migration.md) for more background information.

In reality, this is the controller which can cause quite a lot of network I/O (hence, traffic and costs).
While most of the extension states change rather rarely, the state of the `extensions.gardener.cloud/v1alpha1.Worker` resources can update quite frequently whenever new worker machines join or leave the cluster.
`gardenlet` has to watch and replicate all such state changes to the `ShootState` resource.

In large Gardener installations, this can result in a lot of requests to the `gardener-apiserver` (transitively, also to ETCD of the garden cluster), and it can significantly contribute to the ETCD size (note that ETCD has a practical space limit of 8 GB).
Besides, it increases the memory consumption considerably for all clients working with `ShootState`s.

`ShootState`s were introduced years ago when control plane migration was still in its early phase.
Back then, we wanted to achieve so-called "bad case control plane migrations" (which practically means restoring a shoot control plane without being able to access its current seed cluster (maybe because it's down or destroyed)).
For such scenarios, it is obviously necessary to have to most up-to-date state available in order to perform the restoration in a new seed cluster.
As the current seed cluster is unavailable in this scenario and cannot be accessed, we decided to continuously replicate the state to the garden cluster.

With [Finalise "Bad Case" Control Plane Migration #6302](https://github.com/gardener/gardener/issues/6302), however, this use case has been eliminated.
It was decided that it won't be implemented, especially after [☂️ [GEP-20] Highly Available Seed and Shoot Clusters #6529](https://github.com/gardener/gardener/issues/6529) brings the ability to run seed clusters in a highly available manner (which reduces the chances to ever need a "bad case control plane migration" even further).
With this, it is no longer necessary to continuously replicate the relevant state to the garden cluster, allowing us to drop the `ShootState` API for good.

More generally, even with the "bad case control plane migration" scenario, the `ShootState` API is a design flaw since it was misused for persisting frequently changing data that should better be stored in an object store (similar to ETCD backups).

### Goals

- Reduce network traffic and resulting costs due to continuous replication of state from seed clusters to garden clusters.
- Reduce size of garden cluster's ETCD, and with it the risk to run into its practical space limit of 8 GB.
- Simplify code that was mainly introduced in the context of the "bad case control plane migration".

### Non-Goals

- Adaptation of the ["first use-case"](#summary) of the `ShootState` API (this is handled separately in [Introduce `core.gardener.cloud/v1beta1.InternalSecret` API #7999](https://github.com/gardener/gardener/issues/7999)).
- Provide means to restore a deleted `Shoot` from its persisted state.
- Perform regular backups of the shoot states.

## Proposal

Now that the "bad case control plane migration" scenario has been eliminated, the sole usage of the `ShootState` resource is for the "good case control plane migration".
However, this case is significantly simpler since the current seed cluster is still available.
Hence, when a control plane migration is triggered, `gardenlet` could easily collect the needed data from the current seed cluster, store it in the garden cluster for later usage during the restoration phase, and eventually delete the data again.

We are proposing the following changes:

1. The [`shootstate-extensions`](../../pkg/gardenlet/controller/shootstate/extensions) controller of `gardenlet` will be dropped, i.e., it will no longer replicate the extensions states to the `ShootState` resources in the garden cluster.
2. The [`shootstate-secret`](../../pkg/gardenlet/controller/shootstate/secret) controller will also be dropped as soon as [Introduce `core.gardener.cloud/v1beta1.InternalSecret` API #7999](https://github.com/gardener/gardener/issues/7999) has been implemented.

With this, the `ShootState` API can be dropped entirely.

3. During the `Migrate` phase of a `Shoot`, `gardenlet` will fetch all relevant state in the shoot namespaces of the current seed, similar to what the two controllers are doing as of today:

   1. all labels and data for `Secret`s labeled with `persist=true`.
   2. all `.status.state` and `.status.resources` for resources in the `extensions.gardener.cloud/v1alpha1` API group.

   The `gardenlet` puts this state into a structure similar to the [`core.gardener.cloud/v1beta1.ShootSpec`](https://github.com/gardener/gardener/blob/27b76d8d825461b9edc75c2d4472829e309af05e/pkg/apis/core/v1beta1/types_shootstate.go#L49-L101), marshals it to JSON, and puts it into the data of an [`core.gardener.cloud/v1beta1.InternalSecret`](https://github.com/gardener/gardener/issues/7999) stored in the namespace of the respective `Shoot` in the garden cluster.

4. During the `Restore` phase of a `Shoot`, `gardenlet` will read the created [`core.gardener.cloud/v1beta1.InternalSecret`](https://github.com/gardener/gardener/issues/7999) from the namespace of the respective `Shoot` in the garden cluster, unmarshals its data from JSON to a structure similar to the [`core.gardener.cloud/v1beta1.ShootSpec`](https://github.com/gardener/gardener/blob/27b76d8d825461b9edc75c2d4472829e309af05e/pkg/apis/core/v1beta1/types_shootstate.go#L49-L101), and uses it to populate the state in the new seed cluster just like today.
5. After successful restoration, the created [`core.gardener.cloud/v1beta1.InternalSecret`](https://github.com/gardener/gardener/issues/7999) is deleted again from the namespace of the respective `Shoot` in the garden cluster.

This approach addresses all the concerns and downsides of today's implementation:
Control plane migrations happen rather rarely and usually only take a few minutes.
The shoot state is still replicated to the garden cluster during this process, however it will no longer exist for the entire lifetime of the `Shoot` but only during the short period of the control plane migration (which in reality is actually never executed for most `Shoot`s).
It will also not be continuously updated anymore.
Only when a migration is performed, the current state is collected, and deleted again after the migration has been completed.

In summary:

- the size of the ETCD of the garden cluster will be drastically reduced, especially for large Gardener installations.
- the load on the API server and ETCD of the garden cluster will be drastically reduced, preparing the components for further scale.
- the network traffic and related costs will be drastically reduced.

## Alternatives

Instead of persisting the shoot state via an [`core.gardener.cloud/v1beta1.InternalSecret`](https://github.com/gardener/gardener/issues/7999) to the garden cluster during the control plane migration period, the relevant data could be pushed into the backup buckets that already exist as of today and are used for the ETCD backups.

We have implemented a [PoC](https://github.com/rfranzke/gardener/commits/hackathon/shootstate-s3) during the [Hack the Garden 05/2023 hackathon](https://github.com/gardener-community/hackathon/tree/main/2023-05_Leverkusen) which extends the `extensions.gardener.cloud/v1alpha1` API with two new resources:

- `BackupUpload`s, which can be used to upload arbitrary data to an existing `BackupEntry`.
  <details>
  <summary>Example YAML manifest</summary>

  ```yaml
  apiVersion: extensions.gardener.cloud/v1alpha1
  kind: BackupUpload
  metadata:
    name: example
    namespace: default
  spec:
    entryName: example-entry
    filePath: shootstate.yaml.enc
    type: local
    data: <to-be-uploaded-data>
  ```
  </details>

- `BackupDownload`s, which can be used to download arbitrary data from an existing `BackupEntry`.
  <details>
  <summary>Example YAML manifest</summary>

  ```yaml
  apiVersion: extensions.gardener.cloud/v1alpha1
  kind: BackupDownload
  metadata:
    name: example
    namespace: default
  spec:
    entryName: example-entry
    filePath: shootstate.yaml.enc
    type: local
  status:
    data: <downloaded-data>
  ```
  </details>

Similar to all other resources in the `extensions.gardener.cloud/v1alpha1` API, we could provide respective controllers in the [extensions library](https://github.com/gardener/gardener/tree/master/extensions) such that provider extensions only need to implement their specific business logic (i.e., uploading/downloading to/from their infrastructure-specific object storage).

However, this approach is significantly more expensive in terms of development costs and complexity since the following additional adaptations would be required:

- Introduction of new resources in the `extensions.gardener.cloud/v1alpha1` API which comes with validation, deployment dependencies, etc.
- Introduction of new extension controllers.
- Adaptation of all existing provider extensions with support for `Backup{Bucket,Entry}`s with the implementation for the new `Backup{Up,Down}load` resources.
- Encryption of the shoot state data (since we do not want to store it unencryptedly in the backup buckets).
- Management (incl. rotation) of a respective encryption key.

So far, the only justification for following such a path would be the necessity for regular backups of the shoot states (e.g., every hour or so).
In this case, the shoot states would again be stored in the garden cluster for the entire lifetime of the `Shoot`s.

This alternative approach would still improve a few things like network traffic, etc. since the data would no longer be continuously replicated (only with a much lower frequency compared to today), but eventually today's technical debts remain limiting the garden cluster's scalability.
As no appropriate reasoning for regular backups has been presented yet, they were added as "non-goal" in this GEP.
Hence, implementing this alternative approach is considered unnecessary.

## Future Improvements

With [Move `machine-controller-manager` reconciliation responsibility from extensions to `gardenlet` #7594](https://github.com/gardener/gardener/issues/7594), `gardenlet` will become known of the `machine.sapcloud.io/v1alpha1` API used for managing the worker machines of shoot clusters.

This allows to drop the [`Worker` state reconciler](https://github.com/gardener/gardener/blob/master/extensions/pkg/controller/worker/state_reconciler.go) which currently watches all `MachineDeployment`s, `MachineSet`s, and `Machine`s, and replicates their specifications into the `.status.state` of the related `extensions.gardener.cloud/v1alpha1.Worker` resource.
Instead, `gardenlet` could now collect these resources during the `Migrate` phase itself, effectively compute the necessary `Worker` state, and also restore it during the `Restore` phase.

With this approach, we could also reduce the load on the seed clusters' API servers since it would no longer be necessary to continuously duplicate the machine-related resources into the `Worker` status.
For now, this GEP focuses on the `ShootState` API only, but as soon as [Move `machine-controller-manager` reconciliation responsibility from extensions to `gardenlet` #7594](https://github.com/gardener/gardener/issues/7594) has been implemented, we could start looking into this next-step improvement.
