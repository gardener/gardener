---
title: Improved Usage of the `ShootState` API
gep-number: 22
creation-date: 2023-06-01
status: implementable
authors:
- "@rfranzke"
reviewers:
- "@timebertt"
---

# GEP-22: Improved Usage of the `ShootState` API

## Table of Contents

<!-- TOC -->
- [GEP-22: Improved Usage of the `ShootState` API](#gep-22-improved-usage-of-the-shootstate-api)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
  - [Alternatives](#alternatives)
  - [Future Improvements](#future-improvements)
    - [Eliminating the `Worker` State Reconciler](#eliminating-the-worker-state-reconciler)
    - [Compressing the `ShootState` Data](#compressing-the-shootstate-data)
<!-- TOC -->

## Summary

For each `Shoot` resource, `gardenlet` creates a corresponding `ShootState` resource which has the same lifecycle as the `Shoot`.
Today, such resources are used for two purposes:

1. Exposing the client CA certificate and key of the shoot cluster such that the `gardener-apiserver` is able to issue short-lived client certificates via the [`shoots/adminkubeconfig` subresource](../usage/shoot-basics/shoot_access.md#shootsadminkubeconfig-subresource).
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
With this, it is no longer necessary to continuously replicate the relevant state to the garden cluster, allowing us to reduce the usage of the `ShootState` API.

### Goals

- Reduce network traffic and resulting costs due to continuous replication of state from seed clusters to garden clusters.
- Reduce size of garden cluster's ETCD, and with it the risk to run into its practical space limit of 8 GB.
- Simplify code that was mainly introduced in the context of the "bad case control plane migration".
- Perform regular backups of the shoot states for shoots running on unmanaged seeds.

### Non-Goals

- Adaptation of the ["first use-case"](#summary) of the `ShootState` API (this is handled separately in [Introduce `core.gardener.cloud/v1beta1.InternalSecret` API #7999](https://github.com/gardener/gardener/issues/7999)).
- Provide means to restore a deleted `Shoot` from its persisted state.

## Proposal

Now that the "bad case control plane migration" scenario has been eliminated, the sole usage of the `ShootState` resource is for the "good case control plane migration".
However, this case is significantly simpler since the current seed cluster is still available.
Hence, when a control plane migration is triggered, `gardenlet` could easily collect the needed data from the current seed cluster, store it in the garden cluster for later usage during the restoration phase, and eventually delete the data again.

We are proposing the following changes:

1. The [`shootstate-extensions`](../../pkg/gardenlet/controller/shootstate/extensions) controller of `gardenlet` will be dropped, i.e., it will no longer replicate the extensions states to the `ShootState` resources in the garden cluster.
2. The [`shootstate-secret`](../../pkg/gardenlet/controller/shootstate/secret) controller will also be dropped as soon as [Introduce `core.gardener.cloud/v1beta1.InternalSecret` API #7999](https://github.com/gardener/gardener/issues/7999) has been implemented.

With this, the usage of the `ShootState` API can be drastically reduced.

3. During the `Migrate` phase of a `Shoot`, `gardenlet` will fetch all relevant state in the shoot namespaces of the current seed, similar to what the two controllers are doing as of today:

   1. all labels and data for `Secret`s labeled with `persist=true`.
   2. all `.status.state` and `.status.resources` for resources in the `extensions.gardener.cloud/v1alpha1` API group.

   The `gardenlet` puts this state into regular `core.gardener.cloud/v1beta1.ShootState` resource stored in the namespace of the respective `Shoot` in the garden cluster.

4. During the `Restore` phase of a `Shoot`, `gardenlet` will read the created `core.gardener.cloud/v1beta1.ShootState` from the namespace of the respective `Shoot` in the garden cluster and uses it to populate the state in the new seed cluster just like today.
5. After successful restoration, the created `core.gardener.cloud/v1beta1.ShootState` is deleted again from the namespace of the respective `Shoot` in the garden cluster.

This approach addresses all the concerns and downsides of today's implementation:
Control plane migrations happen rather rarely and usually only take a few minutes.
The shoot state is still replicated to the garden cluster during this process, however it will no longer exist for the entire lifetime of the `Shoot` but only during the short period of the control plane migration (which in reality is actually never executed for most `Shoot`s).
It will also not be continuously updated anymore.
Only when a migration is performed, the current state is collected, and deleted again after the migration has been completed.

> ⚠️There will be one exception to the above which applies for `Shoot`s running on so-called "unmanaged `Seed`s". Such `Seed`s are those not backed by a `seedmanagement.gardener.cloud/v1alpha1.ManagedSeed` object.
>
> For `Shoot`s running on such unmanaged `Seed`s, we cannot make any assumptions about the ETCD backups.
> While we can easily access the ETCD backup of a `ManagedSeed` in case of a disaster in order to get access to the relevant state of a particular `Shoot`, we can't do so for unmanaged `Seed`s.
> 
> Hence, we propose to run a periodic job that backs up the shoot state for `Shoot`s running on unmanaged `Seed`s.
> This periodic job will be implemented as part of a new controller in `gardenlet` which runs according to the configured period (default: `6h`).

In summary:

- the size of the ETCD of the garden cluster will be drastically reduced, especially for large Gardener installations.
- the load on the API server and ETCD of the garden cluster will be drastically reduced, preparing the components for further scale.
- the network traffic and related costs will be drastically reduced.
- only very few `ShootState`s will remain (and those get updated less frequently), since productive Gardener landscapes typically don't run a lot of `Shoot`s on unmanaged `Seed`s.

## Alternatives

Instead of persisting the shoot state via the `core.gardener.cloud/v1beta1.ShootState` resource into ETCD of the garden cluster, the relevant data could be pushed into the backup buckets that already exist as of today and are used for the ETCD backups.

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

Similar to all other resources in the `extensions.gardener.cloud/v1alpha1` API, we could provide respective controllers in the [extensions library](../../extensions) such that provider extensions only need to implement their specific business logic (i.e., uploading/downloading to/from their infrastructure-specific object storage).

However, this approach is significantly more expensive in terms of development costs and complexity since the following additional adaptations would be required:

- Introduction of new resources in the `extensions.gardener.cloud/v1alpha1` API which comes with validation, deployment dependencies, etc.
- Introduction of new extension controllers.
- Adaptation of all existing provider extensions with support for `Backup{Bucket,Entry}`s with the implementation for the new `Backup{Up,Down}load` resources.
- Encryption of the shoot state data (since we do not want to store it as plain text in the backup buckets). With above proposal, encryption is handled by `gardener-apiserver` via the standard `EncryptionConfiguration`.
- Management (incl. rotation) of a respective encryption key.

So far, the only justification for following such a path would be the necessity for regular backups of the shoot states (e.g., every hour or so).
In this case, the shoot states would again be stored in the garden cluster for the entire lifetime of the `Shoot`s.

This alternative approach would still improve a few things like network traffic, etc. since the data would no longer be continuously replicated (only with a much lower frequency compared to today), but eventually today's technical debts remain limiting the garden cluster's scalability.
As no appropriate reasoning for regular backups has been presented yet, they were added as "non-goal" in this GEP.
Hence, implementing this alternative approach is considered unnecessary.

## Future Improvements

### Eliminating the `Worker` State Reconciler

With [Move `machine-controller-manager` reconciliation responsibility from extensions to `gardenlet` #7594](https://github.com/gardener/gardener/issues/7594), `gardenlet` will become aware of the `machine.sapcloud.io/v1alpha1` API used for managing the worker machines of shoot clusters.

This allows to drop the [`Worker` state reconciler](../../extensions/pkg/controller/worker/reconciler.go) which currently watches all `MachineDeployment`s, `MachineSet`s, and `Machine`s, and replicates their specifications into the `.status.state` of the related `extensions.gardener.cloud/v1alpha1.Worker` resource.
Instead, `gardenlet` could then collect these resources during the `Migrate` phase itself, effectively compute the necessary `Worker` state, and also restore it during the `Restore` phase.

With this approach, we could also reduce the load on the seed clusters' API servers since it would no longer be necessary to continuously duplicate the machine-related resources into the `Worker` status.
For now, this GEP focuses on the `ShootState` API only, but as soon as [Move `machine-controller-manager` reconciliation responsibility from extensions to `gardenlet` #7594](https://github.com/gardener/gardener/issues/7594) has been implemented, we could start looking into this next-step improvement.

### Compressing the `ShootState` Data

Typically, the size of a `ShootState` resource is dominated by the `MachineDeployment`s, `MachineSet`s, and `Machine`s.
In case the few remaining `ShootState` resources still grow too large, one could implement compression strategies for the machine state (and maybe other state as well, if necessary) to reduce the overall storage size.

Since we expect only a very small number of `ShootState`s to remain for productive Gardener installations, this optimization is out of scope for now.
