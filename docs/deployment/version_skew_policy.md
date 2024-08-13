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
- other `gardener-apiserver` instances are supported at **1.37** and **1.36**

#### gardener-controller-manager, gardener-scheduler, gardener-admission-controller

`gardener-controller-manager`, `gardener-scheduler`, and `gardener-admission-controller` must not be newer than the `gardener-apiserver` instances they communicate with.
They are expected to match the `gardener-apiserver` minor version, but may be up to one minor version older (to allow live upgrades).

Example:

- `gardener-apiserver` is at **1.37**
- `gardener-controller-manager`, `gardener-scheduler`, and `gardener-admission-controller` are supported at **1.37** and **1.36**

#### gardenlet

- `gardenlet` must not be newer than `gardener-apiserver`
- `gardenlet` may be up to two minor versions older than `gardener-apiserver`

Example:

- `gardener-apiserver` is at **1.37**
- `gardenlet` is supported at **1.37**, **1.36**, and **1.35**

#### gardener-operator

Since `gardener-operator` manages the Gardener control plane components (`gardener-apiserver`, `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`), it follows the same policy as for [`gardener-apiserver`](#gardener-apiserver).

It implements additional start-up checks to ensure adherence to this policy.
Concretely, `gardener-operator` will crash when

- its gets downgraded.
- its version gets upgraded and skips at least one minor version.

### Supported Component Upgrade Order

The supported version skew between components has implications on the order in which components must be upgraded.
This section describes the order in which components must be upgraded to transition an existing Gardener installation from version **1.37** to version **1.38**.

#### gardener-apiserver

Prerequisites:

- In a single-instance setup, the existing `gardener-apiserver` instance is **1.37**.
- In a multi-instance setup, all `gardener-apiserver` instances are at **1.37** or **1.38** (this ensures maximum skew of 1 minor version between the oldest and newest `gardener-apiserver` instance).
- The `gardener-controller-manager`, `gardener-scheduler`, and `gardener-admission-controller` instances that communicate with this `gardener-apiserver` are at version **1.37** (this ensures they are not newer than the existing API server version and are within 1 minor version of the new API server version).
- `gardenlet` instances on all seeds are at version **1.37** or **1.36** (this ensures they are not newer than the existing API server version and are within 2 minor versions of the new API server version).

Actions:

- Upgrade `gardener-apiserver` to **1.38**.

#### gardener-controller-manager, gardener-scheduler, gardener-admission-controller

Prerequisites:

- The `gardener-apiserver` instances these components communicate with are at **1.38** (in multi-instance setups in which these components can communicate with any `gardener-apiserver` instance in the cluster, all `gardener-apiserver` instances must be upgraded before upgrading these components).

Actions:

- Upgrade `gardener-controller-manager`, `gardener-scheduler`, and `gardener-admission-controller` to **1.38**

#### gardenlet

Prerequisites:

- The `gardener-apiserver` instances the `gardenlet` communicates with are at **1.38**.

Actions:

- Optionally upgrade `gardenlet` instances to **1.38** (or they can be left at **1.37** or **1.36**).

> [!WARNING]
> Running a landscape with `gardenlet` instances that are persistently two minor versions behind `gardener-apiserver` means they must be upgraded before the Gardener control plane can be upgraded.

#### gardener-operator

Prerequisites:

- All `gardener-operator` instances are at **1.37**.

Actions:

- Upgrade `gardener-operator` to **1.38**.

## Supported Gardener Extension Versions

Extensions are maintained and released separately and independently of the `gardener/gardener` repository.
Consequently, providing version constraints is not possible in this document.
Sometimes, the documentation of extensions contains compatibility information (e.g., "this extension version is only compatible with Gardener versions higher than **1.80**", see [this example](https://github.com/gardener/gardener-extension-provider-aws#compatibility)).

However, since all extensions typically make use of the [extensions library](../../extensions) ([example](https://github.com/gardener/gardener-extension-provider-aws/blob/cb96b60c970c2e20615dffb3018dc0571cab764d/go.mod#L12)), a general constraint is that _no extension must depend on a version of the extensions library higher than the version of `gardenlet`_.

Example 1:

- `gardener-apiserver` and other Gardener control plane components are at **1.37**.
- All `gardenlet`s are at **1.37**.
- Only extensions are supported which depend on **1.37** or lower of the extensions library.

Example 2:

- `gardener-apiserver` and other Gardener control plane components are at **1.37**.
- Some `gardenlet`s are at **1.37**, others are at **1.36**.
- Only extensions are supported which depend on **1.36** or lower of the extensions library.

## Supported Kubernetes Versions

Please refer to [Supported Kubernetes Versions](../usage/supported_k8s_versions.md).
