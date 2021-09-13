# Feature Gates in Gardener

This page contains an overview of the various feature gates an administrator can specify on different Gardener components.

## Overview

Feature gates are a set of key=value pairs that describe Gardener features. You can turn these features on or off using the a component configuration file for a specific component.

Each Gardener component lets you enable or disable a set of feature gates that are relevant to that component. For example this is the configuration of the [gardenlet](../../example/20-componentconfig-gardenlet.yaml) component.

The following tables are a summary of the feature gates that you can set on different Gardener components.

* The “Since” column contains the Gardener release when a feature is introduced or its release stage is changed.
* The “Until” column, if not empty, contains the last Gardener release in which you can still use a feature gate.
* If a feature is in the Alpha or Beta state, you can find the feature listed in the Alpha/Beta feature gate table.
* If a feature is stable you can find all stages for that feature listed in the Graduated/Deprecated feature gate table.
* The Graduated/Deprecated feature gate table also lists deprecated and withdrawn features.

## Feature gates for Alpha or Beta features

| Feature | Default | Stage | Since | Until |
| --- | --- | --- | --- | --- |
| Logging | `false` | `Alpha` | `0.13` | |
| HVPA | `false` | `Alpha` | `0.31` | |
| HVPAForShootedSeed | `false` | `Alpha` | `0.32` | |
| ManagedIstio | `false` | `Alpha` | `1.5` | `1.18` |
| ManagedIstio | `true` | `Beta` | `1.19` | |
| APIServerSNI | `false` | `Alpha` | `1.7` | `1.18` |
| APIServerSNI | `true` | `Beta` | `1.19` | |
| CachedRuntimeClients | `false` | `Alpha` | `1.7` | |
| SeedChange | `false` | `Alpha` | `1.12` | |
| SeedKubeScheduler | `false` | `Alpha` | `1.15` | |
| ReversedVPN | `false` | `Alpha` | `1.22` | |
| AdminKubeconfigRequest | `false` | `Alpha` | `1.24` | |
| UseDNSRecords | `false` | `Alpha` | `1.27` | |
| DisallowKubeconfigRotationForShootInDeletion | `false` | `Alpha` | `1.28` | `1.31` |
| DisallowKubeconfigRotationForShootInDeletion | `true` | `Beta` | `1.32` | |
| RotateSSHKeypairOnMaintenance | `false` | `Alpha` | `1.28` | |
| DenyInvalidExtensionResources | `false` | `Alpha` | `1.31` | |

## Feature gates for graduated or deprecated features

| Feature | Default | Stage | Since | Until |
| --- | --- | --- | --- | --- |
| NodeLocalDNS | `false` | `Alpha` | `1.7` | |
| NodeLocalDNS | | `Removed` | `1.26` | |
| KonnectivityTunnel | `false` | `Alpha` | `1.6` | |
| KonnectivityTunnel | | `Removed` | `1.27` | |
| MountHostCADirectories | `false` | `Alpha` | `1.11` | `1.25` |
| MountHostCADirectories | `true` | `Beta` | `1.26` | `1.27` |
| MountHostCADirectories | `true` | `GA` | `1.27` | |
| MountHostCADirectories | | `Removed` | `1.30` | |

## Using a feature

A feature can be in *Alpha*, *Beta* or *GA* stage.
An *Alpha* feature means:

* Disabled by default.
* Might be buggy. Enabling the feature may expose bugs.
* Support for feature may be dropped at any time without notice.
* The API may change in incompatible ways in a later software release without notice.
* Recommended for use only in short-lived testing clusters, due to increased
  risk of bugs and lack of long-term support.

A *Beta* feature means:

* Enabled by default.
* The feature is well tested. Enabling the feature is considered safe.
* Support for the overall feature will not be dropped, though details may change.
* The schema and/or semantics of objects may change in incompatible ways in a
  subsequent beta or stable release. When this happens, we will provide instructions
  for migrating to the next version. This may require deleting, editing, and
  re-creating API objects. The editing process may require some thought.
  This may require downtime for applications that rely on the feature.
* Recommended for only non-critical uses because of potential for
  incompatible changes in subsequent releases.

> Please do try *Beta* features and give feedback on them!
> After they exit beta, it may not be practical for us to make more changes.

A *General Availability* (GA) feature is also referred to as a *stable* feature. It means:

* The feature is always enabled; you cannot disable it.
* The corresponding feature gate is no longer needed.
* Stable versions of features will appear in released software for many subsequent versions.

## List of feature gates

* `Logging` enables logging stack for Seed clusters.
* `HVPA` enables simultaneous horizontal and vertical scaling in Seed Clusters.
* `HVPAForShootedSeed` enables simultaneous horizontal and vertical scaling in managed seed (aka "shooted seed") clusters.
* `ManagedIstio` enables a Gardener-tailored [Istio](https://istio.io) in each Seed cluster. Disable this feature if Istio is already installed in the cluster. Istio is not automatically removed if this feature is disabled. See the [detailed documentation](../usage/istio.md) for more information.
* `APIServerSNI` enables only one LoadBalancer to be used for every Shoot cluster API server in a Seed. Enable this feature when `ManagedIstio` is enabled or Istio is manually deployed in Seed cluster. See [GEP-8](../proposals/08-shoot-apiserver-via-sni.md) for more details.
* `MountHostCADirectories` enables mounting common CA certificate directories in the Shoot API server pod that might be required for webhooks or OIDC.
* `CachedRuntimeClients` enables a cache in the controller-runtime clients, that Gardener components use. If disabled all controller-runtime clients will directly talk to the API server instead of relying on a cache. The feature gate can be specified for gardenlet and gardener-controller-manager (and gardener-scheduler for the versions `< 1.29`).
* `SeedChange` enables updating the `spec.seedName` field during shoot validation from a non-empty value in order to trigger shoot control plane migration.
* `SeedKubeScheduler` adds custom `kube-scheduler` in `gardener-kube-scheduler` namespace. It schedules [pods with scheduler name](../concepts/seed-admission-controller.md#mutating-webhooks) `gardener-kube-scheduler` on Nodes with higher resource utilization. It requires Seed cluster with kubernetes version `1.18` or higher.
* `ReversedVPN` reverses the connection setup of the vpn tunnel between the Seed and the Shoot cluster(s). It allows Seed and Shoot clusters to be in different networks with only direct access in one direction (Shoot -> Seed). In addition to that, it reduces the amount of load balancers required, i.e. no load balancers are required for the vpn tunnel anymore. It requires `APIServerSNI` and kubernetes version `1.18` or higher to work. Details can be found in [GEP-14](../proposals/14-reversed-cluster-vpn.md).
* `AdminKubeconfigRequest` enables the `AdminKubeconfigRequest` endpoint on Shoot resources. See [GEP-16](../proposals/16-adminkubeconfig-subresource.md) for more details.
* `UseDNSRecords` enables using `DNSRecord` resources for Gardener DNS records instead of `DNSProvider`, `DNSEntry`, and `DNSOwner` resources. See [Contract: `DNSRecord` resources](../extensions/dnsrecord.md) for more details.
* `DisallowKubeconfigRotationForShootInDeletion` when enabled, does not allow kubeconfig rotation to be requested for shoot cluster that is already in deletion phase, i.e. `metadata.deletionTimestamp` is set.
* `RotateSSHKeypairOnMaintenance` enables SSH keypair rotation in the maintenance controller of the gardener-controller-manager. Details can be found in [GEP-15](../proposals/15-manage-bastions-and-ssh-key-pair-rotation.md).
* `DenyInvalidExtensionResources` causes the `seed-admission-controller` to deny invalid extension resources, instead of just logging validation errors.
