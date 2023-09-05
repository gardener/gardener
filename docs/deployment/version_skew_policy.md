# Version Skew Policy

This document describes the maximum version skew supported between various Gardener components.

## Supported Gardener Versions

Gardener versions are expressed as `x.y.z`, where `x` is the major version, `y` is the minor version, and `z` is the patch version, following Semantic Versioning terminology.

The Gardener project maintains release branches for the three most recent minor releases.

Applicable fixes, including security fixes, may be backported to those three release branches, depending on severity and feasibility.
Patch releases are cut from those branches at a regular cadence, plus additional urgent releases when required.

For more information, see the [Releases document](../development/process.md#releases).

### Supported Version Skew

Technically, we follow the same [policy](https://kubernetes.io/releases/version-skew-policy/) as the Kubernetes project.
However, given that our release cadence is much more frequent compared to Kubernetes (every `14d` vs. every `120d`), in many cases it might be possible to skip versions, though we do not test these upgrade paths.
Consequently, in general it might not work, and to be on the safe side, it is highly recommended to follow the described policy.

🚨 Note that downgrading Gardener versions is generally not tested during development and should be considered unsupported.

#### gardener-apiserver

In multi-instance setups of Gardener, the newest and oldest `gardener-apiserver` instances must be within one minor version.

Example:

- newest `gardener-apiserver` is at **1.37**
- other `gardener-apiserver` instances are supported at **1.37** and **v1.36**

#### gardener-controller-manager, gardener-scheduler, gardener-admission-controller, gardenlet

`gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`, and `gardenlet` must not be newer than the `gardener-apiserver` instances they communicate with.
They are expected to match the `gardener-apiserver` minor version, but may be up to one minor version older (to allow live upgrades).

Example:

- `gardener-apiserver` is at **v1.37**
- `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`, and `gardenlet` are supported at **1.37** and **v1.36**

### Supported Component Upgrade Order

The supported version skew between components has implications on the order in which components must be upgraded.
This section describes the order in which components must be upgraded to transition an existing Gardener installation from version **1.37** to version **1.38**.

#### gardener-apiserver

Prerequisites:

- In a single-instance setup, the existing `gardener-apiserver` instance is **1.37**.
- In a multi-instance setup, all `gardener-apiserver` instances are at **1.37** or **1.38** (this ensures maximum skew of 1 minor version between the oldest and newest `gardener-apiserver` instance).
- The `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`, and `gardenlet` instances that communicate with this `gardener-apiserver` are at version **1.37** (this ensures they are not newer than the existing API server version and are within 1 minor version of the new API server version).

Actions:

- Upgrade `gardener-apiserver` to **1.38**.

#### gardener-controller-manager, gardener-scheduler, gardener-admission-controller, gardenlet

Prerequisites:

- The `gardener-apiserver` instances these components communicate with are at **1.38** (in multi-instance setups in which these components can communicate with any `gardener-apiserver` instance in the cluster, all `gardener-apiserver` instances must be upgraded before upgrading these components)

Actions:

- Upgrade `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`, and `gardenlet` to **1.38**

## Supported Kubernetes Versions

Please refer to [Supported Kubernetes Versions](../usage/supported_k8s_versions.md).
