# Feature Gates in Gardener

This page contains an overview of the various feature gates an administrator can specify on different Gardener components.

## Overview

Feature gates are a set of key=value pairs that describe Gardener features. You can turn these features on or off using the component configuration file for a specific component.

Each Gardener component lets you enable or disable a set of feature gates that are relevant to that component. For example, this is the configuration of the [gardenlet](../../example/20-componentconfig-gardenlet.yaml) component.

The following tables are a summary of the feature gates that you can set on different Gardener components.

* The “Since” column contains the Gardener release when a feature is introduced or its release stage is changed.
* The “Until” column, if not empty, contains the last Gardener release in which you can still use a feature gate.
* If a feature is in the *Alpha* or *Beta* state, you can find the feature listed in the Alpha/Beta feature gate table.
* If a feature is stable you can find all stages for that feature listed in the Graduated/Deprecated feature gate table.
* The Graduated/Deprecated feature gate table also lists deprecated and withdrawn features.

## Feature Gates for Alpha or Beta Features

| Feature                                  | Default | Stage   | Since   | Until   |
|------------------------------------------|---------|---------|---------|---------|
| DefaultSeccompProfile                    | `false` | `Alpha` | `1.54`  |         |
| UseNamespacedCloudProfile                | `false` | `Alpha` | `1.92`  | `1.111` |
| UseNamespacedCloudProfile                | `true`  | `Beta`  | `1.112` |         |
| ShootCredentialsBinding                  | `false` | `Alpha` | `1.98`  | `1.106` |
| ShootCredentialsBinding                  | `true`  | `Beta`  | `1.107` |         |
| NewWorkerPoolHash                        | `false` | `Alpha` | `1.98`  |         |
| NodeAgentAuthorizer                      | `false` | `Alpha` | `1.109` | `1.115` |
| NodeAgentAuthorizer                      | `true`  | `Beta`  | `1.116` |         |
| CredentialsRotationWithoutWorkersRollout | `false` | `Alpha` | `1.112` |         |
| InPlaceNodeUpdates                       | `false` | `Alpha` | `1.113` |         |
| RemoveAPIServerProxyLegacyPort           | `false` | `Alpha` | `1.113` |         |
| IstioTLSTermination                      | `false` | `Alpha` | `1.114` |         |
| CloudProfileCapabilities                 | `false` | `Alpha` | `1.116` |         |

## Feature Gates for Graduated or Deprecated Features

| Feature                                      | Default | Stage        | Since   | Until   |
|----------------------------------------------|---------|--------------|---------|---------|
| NodeLocalDNS                                 | `false` | `Alpha`      | `1.7`   | `1.25`  |
| NodeLocalDNS                                 |         | `Removed`    | `1.26`  |         |
| KonnectivityTunnel                           | `false` | `Alpha`      | `1.6`   | `1.26`  |
| KonnectivityTunnel                           |         | `Removed`    | `1.27`  |         |
| MountHostCADirectories                       | `false` | `Alpha`      | `1.11`  | `1.25`  |
| MountHostCADirectories                       | `true`  | `Beta`       | `1.26`  | `1.27`  |
| MountHostCADirectories                       | `true`  | `GA`         | `1.27`  |         |
| MountHostCADirectories                       |         | `Removed`    | `1.30`  |         |
| DisallowKubeconfigRotationForShootInDeletion | `false` | `Alpha`      | `1.28`  | `1.31`  |
| DisallowKubeconfigRotationForShootInDeletion | `true`  | `Beta`       | `1.32`  | `1.35`  |
| DisallowKubeconfigRotationForShootInDeletion | `true`  | `GA`         | `1.36`  | `1.37`  |
| DisallowKubeconfigRotationForShootInDeletion |         | `Removed`    | `1.38`  |         |
| Logging                                      | `false` | `Alpha`      | `0.13`  | `1.40`  |
| Logging                                      |         | `Removed`    | `1.41`  |         |
| AdminKubeconfigRequest                       | `false` | `Alpha`      | `1.24`  | `1.38`  |
| AdminKubeconfigRequest                       | `true`  | `Beta`       | `1.39`  | `1.41`  |
| AdminKubeconfigRequest                       | `true`  | `GA`         | `1.42`  | `1.49`  |
| AdminKubeconfigRequest                       |         | `Removed`    | `1.50`  |         |
| UseDNSRecords                                | `false` | `Alpha`      | `1.27`  | `1.38`  |
| UseDNSRecords                                | `true`  | `Beta`       | `1.39`  | `1.43`  |
| UseDNSRecords                                | `true`  | `GA`         | `1.44`  | `1.49`  |
| UseDNSRecords                                |         | `Removed`    | `1.50`  |         |
| CachedRuntimeClients                         | `false` | `Alpha`      | `1.7`   | `1.33`  |
| CachedRuntimeClients                         | `true`  | `Beta`       | `1.34`  | `1.44`  |
| CachedRuntimeClients                         | `true`  | `GA`         | `1.45`  | `1.49`  |
| CachedRuntimeClients                         |         | `Removed`    | `1.50`  |         |
| DenyInvalidExtensionResources                | `false` | `Alpha`      | `1.31`  | `1.41`  |
| DenyInvalidExtensionResources                | `true`  | `Beta`       | `1.42`  | `1.44`  |
| DenyInvalidExtensionResources                | `true`  | `GA`         | `1.45`  | `1.49`  |
| DenyInvalidExtensionResources                |         | `Removed`    | `1.50`  |         |
| RotateSSHKeypairOnMaintenance                | `false` | `Alpha`      | `1.28`  | `1.44`  |
| RotateSSHKeypairOnMaintenance                | `true`  | `Beta`       | `1.45`  | `1.47`  |
| RotateSSHKeypairOnMaintenance (deprecated)   | `false` | `Beta`       | `1.48`  | `1.50`  |
| RotateSSHKeypairOnMaintenance (deprecated)   |         | `Removed`    | `1.51`  |         |
| ShootForceDeletion                           | `false` | `Alpha`      | `1.81`  | `1.90`  |
| ShootForceDeletion                           | `true`  | `Beta`       | `1.91`  | `1.110` |
| ShootForceDeletion                           | `true`  | `GA`         | `1.111` |         |
| ShootMaxTokenExpirationOverwrite             | `false` | `Alpha`      | `1.43`  | `1.44`  |
| ShootMaxTokenExpirationOverwrite             | `true`  | `Beta`       | `1.45`  | `1.47`  |
| ShootMaxTokenExpirationOverwrite             | `true`  | `GA`         | `1.48`  | `1.50`  |
| ShootMaxTokenExpirationOverwrite             |         | `Removed`    | `1.51`  |         |
| ShootMaxTokenExpirationValidation            | `false` | `Alpha`      | `1.43`  | `1.45`  |
| ShootMaxTokenExpirationValidation            | `true`  | `Beta`       | `1.46`  | `1.47`  |
| ShootMaxTokenExpirationValidation            | `true`  | `GA`         | `1.48`  | `1.50`  |
| ShootMaxTokenExpirationValidation            |         | `Removed`    | `1.51`  |         |
| WorkerPoolKubernetesVersion                  | `false` | `Alpha`      | `1.35`  | `1.45`  |
| WorkerPoolKubernetesVersion                  | `true`  | `Beta`       | `1.46`  | `1.49`  |
| WorkerPoolKubernetesVersion                  | `true`  | `GA`         | `1.50`  | `1.51`  |
| WorkerPoolKubernetesVersion                  |         | `Removed`    | `1.52`  |         |
| DisableDNSProviderManagement                 | `false` | `Alpha`      | `1.41`  | `1.49`  |
| DisableDNSProviderManagement                 | `true`  | `Beta`       | `1.50`  | `1.51`  |
| DisableDNSProviderManagement                 | `true`  | `GA`         | `1.52`  | `1.59`  |
| DisableDNSProviderManagement                 |         | `Removed`    | `1.60`  |         |
| SecretBindingProviderValidation              | `false` | `Alpha`      | `1.38`  | `1.50`  |
| SecretBindingProviderValidation              | `true`  | `Beta`       | `1.51`  | `1.52`  |
| SecretBindingProviderValidation              | `true`  | `GA`         | `1.53`  | `1.54`  |
| SecretBindingProviderValidation              |         | `Removed`    | `1.55`  |         |
| SeedKubeScheduler                            | `false` | `Alpha`      | `1.15`  | `1.54`  |
| SeedKubeScheduler                            | `false` | `Deprecated` | `1.55`  | `1.60`  |
| SeedKubeScheduler                            |         | `Removed`    | `1.61`  |         |
| ShootCARotation                              | `false` | `Alpha`      | `1.42`  | `1.50`  |
| ShootCARotation                              | `true`  | `Beta`       | `1.51`  | `1.56`  |
| ShootCARotation                              | `true`  | `GA`         | `1.57`  | `1.59`  |
| ShootCARotation                              |         | `Removed`    | `1.60`  |         |
| ShootSARotation                              | `false` | `Alpha`      | `1.48`  | `1.50`  |
| ShootSARotation                              | `true`  | `Beta`       | `1.51`  | `1.56`  |
| ShootSARotation                              | `true`  | `GA`         | `1.57`  | `1.59`  |
| ShootSARotation                              |         | `Removed`    | `1.60`  |         |
| ReversedVPN                                  | `false` | `Alpha`      | `1.22`  | `1.41`  |
| ReversedVPN                                  | `true`  | `Beta`       | `1.42`  | `1.62`  |
| ReversedVPN                                  | `true`  | `GA`         | `1.63`  | `1.69`  |
| ReversedVPN                                  |         | `Removed`    | `1.70`  |         |
| ForceRestore                                 |         | `Removed`    | `1.66`  |         |
| SeedChange                                   | `false` | `Alpha`      | `1.12`  | `1.52`  |
| SeedChange                                   | `true`  | `Beta`       | `1.53`  | `1.68`  |
| SeedChange                                   | `true`  | `GA`         | `1.69`  | `1.72`  |
| SeedChange                                   |         | `Removed`    | `1.73`  |         |
| CopyEtcdBackupsDuringControlPlaneMigration   | `false` | `Alpha`      | `1.37`  | `1.52`  |
| CopyEtcdBackupsDuringControlPlaneMigration   | `true`  | `Beta`       | `1.53`  | `1.68`  |
| CopyEtcdBackupsDuringControlPlaneMigration   | `true`  | `GA`         | `1.69`  | `1.72`  |
| CopyEtcdBackupsDuringControlPlaneMigration   |         | `Removed`    | `1.73`  |         |
| ManagedIstio                                 | `false` | `Alpha`      | `1.5`   | `1.18`  |
| ManagedIstio                                 | `true`  | `Beta`       | `1.19`  |         |
| ManagedIstio                                 | `true`  | `Deprecated` | `1.48`  | `1.69`  |
| ManagedIstio                                 |         | `Removed`    | `1.70`  |         |
| APIServerSNI                                 | `false` | `Alpha`      | `1.7`   | `1.18`  |
| APIServerSNI                                 | `true`  | `Beta`       | `1.19`  |         |
| APIServerSNI                                 | `true`  | `Deprecated` | `1.48`  | `1.72`  |
| APIServerSNI                                 |         | `Removed`    | `1.73`  |         |
| HAControlPlanes                              | `false` | `Alpha`      | `1.49`  | `1.70`  |
| HAControlPlanes                              | `true`  | `Beta`       | `1.71`  | `1.72`  |
| HAControlPlanes                              | `true`  | `GA`         | `1.73`  | `1.73`  |
| HAControlPlanes                              |         | `Removed`    | `1.74`  |         |
| FullNetworkPoliciesInRuntimeCluster          | `false` | `Alpha`      | `1.66`  | `1.70`  |
| FullNetworkPoliciesInRuntimeCluster          | `true`  | `Beta`       | `1.71`  | `1.72`  |
| FullNetworkPoliciesInRuntimeCluster          | `true`  | `GA`         | `1.73`  | `1.73`  |
| FullNetworkPoliciesInRuntimeCluster          |         | `Removed`    | `1.74`  |         |
| DisableScalingClassesForShoots               | `false` | `Alpha`      | `1.73`  | `1.78`  |
| DisableScalingClassesForShoots               | `true`  | `Beta`       | `1.79`  | `1.80`  |
| DisableScalingClassesForShoots               | `true`  | `GA`         | `1.81`  | `1.81`  |
| DisableScalingClassesForShoots               |         | `Removed`    | `1.82`  |         |
| ContainerdRegistryHostsDir                   | `false` | `Alpha`      | `1.77`  | `1.85`  |
| ContainerdRegistryHostsDir                   | `true`  | `Beta`       | `1.86`  | `1.86`  |
| ContainerdRegistryHostsDir                   | `true`  | `GA`         | `1.87`  | `1.87`  |
| ContainerdRegistryHostsDir                   |         | `Removed`    | `1.88`  |         |
| WorkerlessShoots                             | `false` | `Alpha`      | `1.70`  | `1.78`  |
| WorkerlessShoots                             | `true`  | `Beta`       | `1.79`  | `1.85`  |
| WorkerlessShoots                             | `true`  | `GA`         | `1.86`  | `1.87`  |
| WorkerlessShoots                             |         | `Removed`    | `1.88`  |         |
| MachineControllerManagerDeployment           | `false` | `Alpha`      | `1.73`  |         |
| MachineControllerManagerDeployment           | `true`  | `Beta`       | `1.81`  | `1.81`  |
| MachineControllerManagerDeployment           | `true`  | `GA`         | `1.82`  | `1.91`  |
| MachineControllerManagerDeployment           |         | `Removed`    | `1.92`  |         |
| APIServerFastRollout                         | `true`  | `Beta`       | `1.82`  | `1.89`  |
| APIServerFastRollout                         | `true`  | `GA`         | `1.90`  | `1.91`  |
| APIServerFastRollout                         |         | `Removed`    | `1.92`  |         |
| UseGardenerNodeAgent                         | `false` | `Alpha`      | `1.82`  | `1.88`  |
| UseGardenerNodeAgent                         | `true`  | `Beta`       | `1.89`  | `1.89`  |
| UseGardenerNodeAgent                         | `true`  | `GA`         | `1.90`  | `1.91`  |
| UseGardenerNodeAgent                         |         | `Removed`    | `1.92`  |         |
| CoreDNSQueryRewriting                        | `false` | `Alpha`      | `1.55`  | `1.95`  |
| CoreDNSQueryRewriting                        | `true`  | `Beta`       | `1.96`  | `1.96`  |
| CoreDNSQueryRewriting                        | `true`  | `GA`         | `1.97`  | `1.100` |
| CoreDNSQueryRewriting                        |         | `Removed`    | `1.101` |         |
| MutableShootSpecNetworkingNodes              | `false` | `Alpha`      | `1.64`  | `1.95`  |
| MutableShootSpecNetworkingNodes              | `true`  | `Beta`       | `1.96`  | `1.96`  |
| MutableShootSpecNetworkingNodes              | `true`  | `GA`         | `1.97`  | `1.100` |
| MutableShootSpecNetworkingNodes              |         | `Removed`    | `1.101` |         |
| VPAForETCD                                   | `false` | `Alpha`      | `1.94`  | `1.96`  |
| VPAForETCD                                   | `true`  | `Beta`       | `1.97`  | `1.104` |
| VPAForETCD                                   | `true`  | `GA`         | `1.105` | `1.108` |
| VPAForETCD                                   |         | `Removed`    | `1.109` |         |
| VPAAndHPAForAPIServer                        | `false` | `Alpha`      | `1.95`  | `1.100` |
| VPAAndHPAForAPIServer                        | `true`  | `Beta`       | `1.101` | `1.104` |
| VPAAndHPAForAPIServer                        | `true`  | `GA`         | `1.105` | `1.108` |
| VPAAndHPAForAPIServer                        |         | `Removed`    | `1.109` |         |
| HVPA                                         | `false` | `Alpha`      | `0.31`  | `1.105` |
| HVPA                                         | `false` | `Deprecated` | `1.106` | `1.108` |
| HVPA                                         |         | `Removed`    | `1.109` |         |
| HVPAForShootedSeed                           | `false` | `Alpha`      | `0.32`  | `1.105` |
| HVPAForShootedSeed                           | `false` | `Deprecated` | `1.106` | `1.108` |
| HVPAForShootedSeed                           |         | `Removed`    | `1.109` |         |
| IPv6SingleStack                              | `false` | `Alpha`      | `1.63`  |         |
| IPv6SingleStack                              |         | `Removed`    | `1.107` |         |
| ShootManagedIssuer                           | `false` | `Alpha`      | `1.93`  | `1.110` |
| ShootManagedIssuer                           |         | `Removed`    | `1.111` |         |
| NewVPN                                       | `false` | `Alpha`      | `1.104` | `1.114` |
| NewVPN                                       | `true`  | `Beta`       | `1.115` | `1.115` |
| NewVPN                                       | `true`  | `GA`         | `1.116` |         |

## Using a Feature

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

| Feature                                  | Relevant Components                | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
|------------------------------------------|------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| DefaultSeccompProfile                    | `gardenlet`, `gardener-operator`   | Enables the defaulting of the seccomp profile for Gardener managed workload in the garden or seed to `RuntimeDefault`.                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ShootForceDeletion                       | `gardener-apiserver`               | Allows forceful deletion of Shoots by annotating them with the `confirmation.gardener.cloud/force-deletion` annotation.                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| UseNamespacedCloudProfile                | `gardener-apiserver`               | Enables usage of `NamespacedCloudProfile`s in `Shoot`s.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| ShootManagedIssuer                       | `gardenlet`                        | Enables the shoot managed issuer functionality described in GEP 24.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ShootCredentialsBinding                  | `gardener-apiserver`               | Enables usage of `CredentialsBindingName` in `Shoot`s.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| NewWorkerPoolHash                        | `gardenlet`                        | Enables usage of the new worker pool hash calculation. The new calculation supports rolling worker pools if `kubeReserved`, `systemReserved`, `evictionHard` or `cpuManagerPolicy` in the `kubelet` configuration are changed. All provider extensions must be upgraded to support this feature first. Existing worker pools are not immediately migrated to the new hash variant, since this would trigger the replacement of all nodes. The migration happens when a rolling update is triggered according to the old or new hash version calculation. |
| NewVPN                                   | `gardenlet`                        | Enables usage of the new implementation of the VPN (go rewrite) using an IPv6 transfer network.                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| NodeAgentAuthorizer                      | `gardenlet`, `gardener-node-agent` | Enables authorization of gardener-node-agent to `kube-apiserver` of shoot clusters using an authorization webhook. It restricts the permissions of each gardener-node-agent instance to the objects belonging to its own node only.                                                                                                                                                                                                                                                                                                                      |
| CredentialsRotationWithoutWorkersRollout | `gardener-apiserver`               | CredentialsRotationWithoutWorkersRollout enables starting the credentials rotation without immediately causing a rolling update of all worker nodes. Instead, the rolling update can be triggered manually by the user at a later point in time of their convenience. This should only be enabled when all deployed provider extensions vendor at least `gardener/gardener@v1.111+`.                                                                                                                                                                     |
| InPlaceNodeUpdates                       | `gardener-apiserver`               | Enables setting the update strategy of worker pools to `AutoInPlaceUpdate` or `ManualInPlaceUpdate` in the Shoot API.                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| RemoveAPIServerProxyLegacyPort           | `gardenlet`                        | Disables the unused proxy port (8443) on the istio-ingressgateway Services. Operators can choose to remove the legacy apiserver-proxy port as soon as all shoots have switched to the new apiserver-proxy configuration. They might want to do so if they activate the ACL extension, which is vulnerable to proxy protocol headers of untrusted clients on the apiserver-proxy port.                                                                                                                                                                    |
| IstioTLSTermination                      | `gardenlet`, `gardener-operator`   | Enables TLS termination for the Istio Ingress Gateway instead of TLS termination at the kube-apiserver. It allows load-balancing of requests to the kube-apiserver on request level instead of connection level.                                                                                                                                                                                                                                                                                                                                         |
| CloudProfileCapabilities                 | `gardener-apiserver`               | Enables the usage of capabilities in the `CloudProfile`. Capabilities are used to create a relation between machineTypes and machineImages. It allows to validate worker groups of a shoot ensuring the selected image and machine combination will boot up successfully. Capabilities are also used to determine valid upgrade paths during automated maintenance operation.                                                                                                                                                                              |
