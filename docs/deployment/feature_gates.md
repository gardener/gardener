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
| ManagedIstio | `false` | `Alpha` | `1.5` | |
| APIServerSNI | `false` | `Alpha` | `1.7` | |
| MountHostCADirectories | `false` | `Alpha` | `1.11.0` | |
| SeedChange | `false` | `Alpha` | `1.12.0` | |

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
* `HVPAForShootedSeed`  enables simultaneous horizontal and vertical scaling in shooted Seed clusters.
* `ManagedIstio` enables a Gardener-tailored [Istio](https://istio.io) in each Seed cluster. Disable this feature if Istio is already installed in the cluster. Istio is not automatically removed if this feature is disabled. See the [detailed documentation](../usage/istio.md) for more information.
* `APIServerSNI` enables only one LoadBalancer to be used for every Shoot cluster API server in a Seed. Enable this feature when `ManagedIstio` is enabled or Istio is manually deployed in Seed cluster. See [GEP-8](../proposals/08-shoot-apiserver-via-sni.md) for more details.
* `MountHostCADirectories` enables mounting common CA certificate directories in the Shoot API server pod that might be required for webhooks or OIDC. 
* `SeedChange` enables updating the `spec.seedName` field during shoot validation from a non-empty value in order to trigger shoot control plane migration.
