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

| Feature                                      | Default | Stage   | Since  | Until  |
| -------------------------------------------- | ------- | ------- | ------ | ------ |
| HVPA                                         | `false` | `Alpha` | `0.31` |        |
| HVPAForShootedSeed                           | `false` | `Alpha` | `0.32` |        |
| ManagedIstio                                 | `false` | `Alpha` | `1.5`  | `1.18` |
| ManagedIstio                                 | `true`  | `Beta`  | `1.19` |        |
| APIServerSNI                                 | `false` | `Alpha` | `1.7`  | `1.18` |
| APIServerSNI                                 | `true`  | `Beta`  | `1.19` |        |
| SeedChange                                   | `false` | `Alpha` | `1.12` |        |
| SeedKubeScheduler                            | `false` | `Alpha` | `1.15` |        |
| ReversedVPN                                  | `false` | `Alpha` | `1.22` | `1.41` |
| ReversedVPN                                  | `true`  | `Beta`  | `1.42` |        |
| RotateSSHKeypairOnMaintenance                | `false` | `Alpha` | `1.28` |        |
| WorkerPoolKubernetesVersion                  | `false` | `Alpha` | `1.35` |        |
| CopyEtcdBackupsDuringControlPlaneMigration   | `false` | `Alpha` | `1.37` |        |
| SecretBindingProviderValidation              | `false` | `Alpha` | `1.38` |        |
| ForceRestore                                 | `false` | `Alpha` | `1.39` |        |
| DisableDNSProviderManagement                 | `false` | `Alpha` | `1.41` |        |
| ShootCARotation                              | `false` | `Alpha` | `1.42` |        |
| ShootMaxTokenExpirationOverwrite             | `false` | `Alpha` | `1.43` | `1.44` |
| ShootMaxTokenExpirationOverwrite             | `true`  | `Beta`  | `1.45` |        |
| ShootMaxTokenExpirationValidation            | `false` | `Alpha` | `1.43` |        |

## Feature gates for graduated or deprecated features

| Feature                                      | Default | Stage     | Since  | Until  |
| -------------------------------------------- | ------- | --------- | ------ | ------ |
| NodeLocalDNS                                 | `false` | `Alpha`   | `1.7`  |        |
| NodeLocalDNS                                 |         | `Removed` | `1.26` |        |
| KonnectivityTunnel                           | `false` | `Alpha`   | `1.6`  |        |
| KonnectivityTunnel                           |         | `Removed` | `1.27` |        |
| MountHostCADirectories                       | `false` | `Alpha`   | `1.11` | `1.25` |
| MountHostCADirectories                       | `true`  | `Beta`    | `1.26` | `1.27` |
| MountHostCADirectories                       | `true`  | `GA`      | `1.27` |        |
| MountHostCADirectories                       |         | `Removed` | `1.30` |        |
| DisallowKubeconfigRotationForShootInDeletion | `false` | `Alpha`   | `1.28` | `1.31` |
| DisallowKubeconfigRotationForShootInDeletion | `true`  | `Beta`    | `1.32` | `1.35` |
| DisallowKubeconfigRotationForShootInDeletion | `true`  | `GA`      | `1.36` |        |
| DisallowKubeconfigRotationForShootInDeletion |         | `Removed` | `1.38` |        |
| Logging                                      | `false` | `Alpha`   | `0.13` | `1.40` |
| Logging                                      | `false` | `Removed` | `1.41` |        |
| AdminKubeconfigRequest                       | `false` | `Alpha`   | `1.24` | `1.38` |
| AdminKubeconfigRequest                       | `true`  | `Beta`    | `1.39` | `1.41` |
| AdminKubeconfigRequest                       | `true`  | `GA`      | `1.42` |        |
| UseDNSRecords                                | `false` | `Alpha`   | `1.27` | `1.38` |
| UseDNSRecords                                | `true`  | `Beta`    | `1.39` | `1.43` |
| UseDNSRecords                                | `true`  | `GA`      | `1.44` |        |
| CachedRuntimeClients                         | `false` | `Alpha`   | `1.7`  | `1.33` |
| CachedRuntimeClients                         | `true`  | `Beta`    | `1.34` | `1.44` |
| CachedRuntimeClients                         | `true`  | `GA`      | `1.45` |        |
| DenyInvalidExtensionResources                | `false` | `Alpha`   | `1.31` | `1.41` |
| DenyInvalidExtensionResources                | `true`  | `Beta`    | `1.42` | `1.44` |
| DenyInvalidExtensionResources                | `true`  | `GA`      | `1.45` |        |

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

## List of Feature Gates

| Feature                                    | Relevant Components                                              | Description |
| ------------------------------------------ | ---------------------------------------------------------------- |  -----------|
| HVPA                                       | `gardenlet`                                                      | Enables simultaneous horizontal and vertical scaling in Seed Clusters. |
| HVPAForShootedSeed                         | `gardenlet`                                                      | Enables simultaneous horizontal and vertical scaling in managed seed (aka "shooted seed") clusters. |
| ManagedIstio                               | `gardenlet`                                                      | Enables a Gardener-tailored [Istio](https://istio.io) in each Seed cluster. Disable this feature if Istio is already installed in the cluster. Istio is not automatically removed if this feature is disabled. See the [detailed documentation](../usage/istio.md) for more information. |
| APIServerSNI                               | `gardenlet`                                                      | Enables only one LoadBalancer to be used for every Shoot cluster API server in a Seed. Enable this feature when `ManagedIstio` is enabled or Istio is manually deployed in Seed cluster. See [GEP-8](../proposals/08-shoot-apiserver-via-sni.md) for more details. |
| CachedRuntimeClients                       | `gardener-controller-manager`, `gardenlet`                       | Enables a cache in the controller-runtime clients, that Gardener components use. The feature gate can be specified for gardenlet and gardener-controller-manager (and gardener-scheduler for the versions `< 1.29`). |
| SeedChange                                 | `gardener-apiserver`                                             | Enables updating the `spec.seedName` field during shoot validation from a non-empty value in order to trigger shoot control plane migration. |
| SeedKubeScheduler                          | `gardenlet`                                                      | Adds custom `kube-scheduler` in `gardener-kube-scheduler` namespace. It schedules [pods with scheduler name](../concepts/seed-admission-controller.md#mutating-webhooks) `gardener-kube-scheduler` on Nodes with higher resource utilization. It requires Seed cluster with kubernetes version `1.18` or higher. |
| ReversedVPN                                | `gardenlet`                                                      | Reverses the connection setup of the vpn tunnel between the Seed and the Shoot cluster(s). It allows Seed and Shoot clusters to be in different networks with only direct access in one direction (Shoot -> Seed). In addition to that, it reduces the amount of load balancers required, i.e. no load balancers are required for the vpn tunnel anymore. It requires `APIServerSNI` and kubernetes version `1.18` or higher to work. Details can be found in [GEP-14](../proposals/14-reversed-cluster-vpn.md). |
| AdminKubeconfigRequest                     | `gardener-apiserver`                                             | Enables the `AdminKubeconfigRequest` endpoint on Shoot resources. See [GEP-16](../proposals/16-adminkubeconfig-subresource.md) for more details. |
| UseDNSRecords                              | `gardener-apiserver`, `gardener-controller-manager`, `gardenlet` | Enables using `DNSRecord` resources for Gardener DNS records instead of `DNSProvider`, `DNSEntry`, and `DNSOwner` resources. See [Contract: `DNSRecord` resources](../extensions/dnsrecord.md) for more details. |
| RotateSSHKeypairOnMaintenance              | `gardener-controller-manager`                                    | Enables SSH keypair rotation in the maintenance controller of the gardener-controller-manager. Details can be found in [GEP-15](../proposals/15-manage-bastions-and-ssh-key-pair-rotation.md). |
| DenyInvalidExtensionResources              | `gardenlet`                                                      | Causes the `seed-admission-controller` to deny invalid extension resources, instead of just logging validation errors. |
| WorkerPoolKubernetesVersion                | `gardener-apiserver`                                             | Allows to overwrite the Kubernetes version used for shoot clusters per worker pool (see [this document](../usage/worker_pool_k8s_versions.md)) |
| CopyEtcdBackupsDuringControlPlaneMigration | `gardenlet`                                                      | Enables the copy of etcd backups from the object store of the source seed to the object store of the destination seed during control plane migration. |
| SecretBindingProviderValidation            | `gardener-apiserver`                                             | Enables validations on Gardener API server that:<br>- requires the provider type of a SecretBinding to be set (on SecretBinding creation)<br>- requires the SecretBinding provider type to match the Shoot provider type (on Shoot creation)<br>- enforces immutability on the provider type of a SecretBinding |
| ForceRestore                               | `gardenlet`                                                      | Enables forcing the shoot's restoration to the destination seed during control plane migration if the preparation for migration in the source seed is not finished after a certain grace period and is considered unlikely to succeed (falling back to the [control plane migration "bad case" scenario](../proposals/17-shoot-control-plane-migration-bad-case.md)). If you enable this feature gate, make sure to also enable `UseDNSRecords` and `CopyEtcdBackupsDuringControlPlaneMigration`. |
| DisableDNSProviderManagement               | `gardenlet`                                                      | Disables management of `dns.gardener.cloud/v1alpha1.DNSProvider` resources. In this case, the `shoot-dns-service` extension will take this over if it is installed. This feature is only effective if the feature `UseDNSRecords` is `true`. |
| ShootCARotation                            | `gardener-apiserver`, `gardenlet`                                | Enables the feature to trigger automated CA rotation for shoot clusters. |
| ShootMaxTokenExpirationOverwrite           | `gardener-apiserver`                                             | Makes the Gardener API server overwriting values in the `.spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration` field of Shoot specifications to<br>- be at least 720h (30d) when the current value is lower<br>- be at most 2160h (90d) when the current value is higher<br>before persisting the object to etcd. |
| ShootMaxTokenExpirationValidation          | `gardener-apiserver`                                             | Enables validations on Gardener API server that enforce that the value of the `.spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration` field<br>- is at least 720h (30d).<br>- is at most 2160h (90d).<br>Only enable this after `ShootMaxTokenExpirationOverwrite` is enabled and all shoots got updated accordingly. |
