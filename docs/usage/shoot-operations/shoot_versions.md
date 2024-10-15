---
title: Shoot Kubernetes and Operating System Versioning in Gardener
---

# Shoot Kubernetes and Operating System Versioning in Gardener

## Motivation

On the one hand-side, Gardener is responsible for managing the Kubernetes and the Operating System (OS) versions of its Shoot clusters.
On the other hand-side, Gardener needs to be configured and updated based on the availability and support of the Kubernetes and Operating System version it provides.
For instance, the Kubernetes community releases **minor** versions roughly every three months and usually maintains **three minor** versions (the current and the last two) with bug fixes and security updates.
Patch releases are done more frequently.

When using the term `Machine image` in the following, we refer to the OS version that comes with the machine image of the node/worker pool of a Gardener Shoot cluster.
As such, we are not referring to the `CloudProvider` specific machine image like the [`AMI`](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) for AWS.
For more information on how Gardener maps machine image versions to `CloudProvider` specific machine images, take a look at the individual gardener extension providers, such as the [provider for AWS](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/operations/operations.md).

Gardener should be configured accordingly to reflect the "logical state" of a version.
It should be possible to define the Kubernetes or Machine image versions that still receive bug fixes and security patches, and also vice-versa to define the version that are out-of-maintenance and are potentially vulnerable.
Moreover, this allows Gardener to "understand" the current state of a version and act upon it (more information in the following sections).

## Overview

**As a Gardener operator**:

- I can classify a version based on it's logical state (`preview`, `supported`, `deprecated`, and `expired`; see [Version Classification](#version-classifications)).
- I can define which Machine image and Kubernetes versions are eligible for the auto update of clusters during the maintenance time.
- I can define a moment in time when Shoot clusters are forcefully migrated off a certain version (through an `expirationDate`).
- I can define an update path for machine images for auto and force updates; see [Update path for machine image versions](#update-path-for-machine-image-versions)).
- I can disallow the creation of clusters having a certain version (think of severe security issues).

**As an end-user/Shoot owner of Gardener**:

- I can get information about which Kubernetes and Machine image versions exist and their classification.
- I can determine the time when my Shoot clusters Machine image and Kubernetes version will be forcefully updated to the next patch or minor version (in case the cluster is running a deprecated version with an expiration date).
- I can get this information via API from the `CloudProfile`.

## Version Classifications

Administrators can classify versions into four distinct "logical states": `preview`, `supported`, `deprecated`, and `expired`.
The version classification serves as a "point-of-reference" for end-users and also has implications during shoot creation and the maintenance time.

If a version is unclassified, Gardener cannot make those decision based on the "logical state".
Nevertheless, Gardener can operate without version classifications and can be added at any time to the Kubernetes and machine image versions in the `CloudProfile`.

As a best practice, versions usually start with the classification `preview`, then are promoted to `supported`, eventually `deprecated` and finally `expired`.
This information is programmatically available in the `CloudProfiles` of the Garden cluster.

- **preview:** A `preview` version is a new version that has not yet undergone thorough testing, possibly a new release, and needs time to be validated.
Due to its short early age, there is a higher probability of undiscovered issues and is therefore not yet recommended for production usage.
A Shoot does not update (neither `auto-update` or `force-update`) to a `preview` version during the maintenance time.
Also, `preview` versions are not considered for the defaulting to the highest available version when deliberately omitting the patch version during Shoot creation.
Typically, after a fresh release of a new Kubernetes (e.g., v1.25.0) or Machine image version (e.g., suse-chost 15.4.20220818), the operator tags it as `preview` until they have gained sufficient experience and regards this version to be reliable.
After the operator has gained sufficient trust, the version can be manually promoted to `supported`.

- **supported:** A `supported` version is the recommended version for new and existing Shoot clusters. This is the version that new Shoot clusters should use and existing clusters should update to.
Typically for Kubernetes versions, the latest Kubernetes patch versions of the actual (if not still in `preview`) and the last 3 minor Kubernetes versions are maintained by the community. An operator could define these versions as being `supported` (e.g., v1.27.6, v1.26.10, and v1.25.12).

- **deprecated:** A `deprecated` version is a version that approaches the end of its lifecycle and can contain issues which are probably resolved in a supported version.
New Shoots should not use this version anymore.
Existing Shoots will be updated to a newer version if `auto-update` is enabled (`.spec.maintenance.autoUpdate.kubernetesVersion` for Kubernetes version `auto-update`, or `.spec.maintenance.autoUpdate.machineImageVersion` for machine image version `auto-update`).
Using automatic upgrades, however, does not guarantee that a Shoot runs a non-deprecated version, as the latest version (overall or of the minor version) can be deprecated as well.
Deprecated versions **should** have an expiration date set for eventual expiration.

- **expired:** An `expired` versions has an expiration date (based on the [Golang time package](https://golang.org/src/time/time.go)) in the past.
New clusters with that version cannot be created and existing clusters are forcefully migrated to a higher version during the maintenance time.

Below is an example how the relevant section of the `CloudProfile` might look like:

``` yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: alicloud
spec:
  kubernetes:
    versions:
      - classification: preview
        version: 1.27.0
      - classification: preview
        version: 1.26.3
      - classification: supported
        version: 1.26.2
      - classification: preview
        version: 1.25.5
      - classification: supported
        version: 1.25.4
      - classification: supported
        version: 1.24.6
      - classification: deprecated
        expirationDate: "2022-11-30T23:59:59Z"
        version: 1.24.5
```

## Automatic Version Upgrades 

There are two ways, the Kubernetes version of the control plane as well as the Kubernetes and machine image version of a worker pool can be upgraded: `auto update` and `forceful` update.
See [Automatic Version Updates](../shoot/shoot_maintenance.md#automatic-version-updates) for how to enable `auto updates` for Kubernetes or machine image versions on the Shoot cluster.

If a Shoot is running a version after its expiration date has passed, it will be forcefully updated during its maintenance time.
This happens **even if the owner has opted out of automatic cluster updates!**

**When an auto update is triggered?**:
- The `Shoot` has auto-update enabled and the version is not the *latest eligible version* for the auto-update. Please note that this *latest version* that qualifies for an auto-update is not necessarily the overall latest version in the CloudProfile:
   - For Kubernetes version, the latest eligible version for auto-updates is the latest patch version of the current minor.
   - For machine image version, the latest eligible version for auto-updates is controlled by the `updateStrategy` field of the machine image in the CloudProfile.
- The `Shoot` has auto-update disabled and the version is either expired or does not exist. 

The auto update can fail if the version is already on the *latest eligible version* for the auto-update. A failed auto update triggers a **force update**.
The force and auto update path for Kubernetes and machine image versions differ slightly and are described in more detail below.

**Update rules for both Kubernetes and machine image versions**
- Both auto and force update first try to update to the latest patch version of the same minor.
- An auto update prefers supported versions over deprecated versions. If there is a lower supported version and a higher deprecated version, auto update will pick the supported version. If all qualifying versions are deprecated, update to the latest deprecated version.
- An auto update never updates to an expired version.
- A force update prefers to update to not-expired versions. If all qualifying versions are expired, update to the latest expired version.  Please note that therefore **multiple consecutive version upgrades** are possible. In this case, the version is again upgraded in the **next** maintenance time.

### Update path for machine image versions

Administrators can define three different **update strategies** (field `updateStrategy`) for machine images in the CloudProfile: `patch`, `minor`, `major (default)`. This is to accommodate the different version schemes of Operating Systems (e.g. Gardenlinux only updates major and minor versions with occasional patches).
- `patch`: update to the latest patch version of the current minor version. When using an expired version: force update to the latest patch of the current minor. If already on the latest patch version, then force update to the next higher (not necessarily +1) minor version.
- `minor`: update to the latest minor and patch version. When using an expired version: force update to the latest minor and patch of the current major. If already on the latest minor and patch of the current major, then update to the next higher (not necessarily +1) major version.
- `major`: always update to the overall latest version. This is the legacy behavior for automatic machine image version upgrades. Force updates are not possible and will fail if the latest version in the CloudProfile for that image is expired (EOL scenario).

Example configuration in the CloudProfile:

```yaml
machineImages:
  - name: gardenlinux
    updateStrategy: minor
    versions:
     - version: 1096.1.0
     - version: 934.8.0
     - version: 934.7.0
  - name: suse-chost
    updateStrategy: patch
    versions:
    - version: 15.3.20220818 
    - version: 15.3.20221118
```

Please note that force updates for machine images can skip minor versions (strategy: patch) or major versions (strategy: minor) if the next minor/major version has no qualifying versions (only `preview` versions).

### Update path for Kubernetes versions

For **Kubernetes versions**, the auto update picks the latest `non-preview` patch version of the current minor version.

If the cluster is already on the latest patch version and the latest patch version is also expired,
it will continue with the latest patch version of the **next consecutive minor (minor +1) Kubernetes version**,
so **it will result in an update of a minor Kubernetes version!**

Kubernetes "minor version jumps" are not allowed - meaning to skip the update to the consecutive minor version and directly update to any version after that.
For instance, the version `1.24.x` can only update to a version `1.25.x`, not to `1.26.x` or any other version.
This is because Kubernetes does not guarantee upgradability in this case, leading to possibly broken Shoot clusters.
The administrator has to set up the `CloudProfile` in such a way that consecutive Kubernetes minor versions are available.
Otherwise, Shoot clusters will fail to upgrade during the maintenance time.

Consider the `CloudProfile` below with a Shoot using the Kubernetes version `1.24.12`.
Even though the version is `expired`, due to missing `1.25.x` versions, the Gardener Controller Manager cannot upgrade the Shoot's Kubernetes version.

```yaml
spec:
  kubernetes:
    versions:
    - version: 1.26.10
    - version: 1.26.9
    - version: 1.24.12
      expirationDate: "<expiration date in the past>"
```

The `CloudProfile` must specify versions `1.25.x` of the **consecutive** minor version.
Configuring the `CloudProfile` in such a way, the Shoot's Kubernetes version will be upgraded to version `1.25.10` in the next maintenance time.

```yaml
spec:
  kubernetes:
    versions:
    - version: 1.26.9
    - version: 1.25.10
    - version: 1.25.9
    - version: 1.24.12
      expirationDate: "<expiration date in the past>"
```

## Version Requirements (Kubernetes and Machine Image)

The Gardener API server enforces the following requirements for versions:

- A version that is in use by a Shoot cannot be deleted from the `CloudProfile`.
- Creating a new version with expiration date in the past is not allowed.
- There can be only one `supported` version per minor version.
- The latest Kubernetes version cannot have an expiration date.
  - NOTE: The latest version for a machine image can have an expiration date. [*]

<sub>[*] Useful for cases in which support for a given machine image needs to be deprecated and removed (for example, the machine image reaches end of life).</sub>

## Related Documentation

You might want to read about the [Shoot Updates and Upgrades](shoot_updates.md) procedures to get to know the effects of such operations.
