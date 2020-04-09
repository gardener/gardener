# Kubernetes and Machine image versions in Gardener

## Motivation

On the one hand-side Gardener is responsible for managing the Kubernetes and Machine image versions of its Shoot clusters.
On the other hand-side, Gardener needs to be configured and updated based on the availability and support of the Kubernetes and Machine image version it provides.

For instance the Kubernetes community releases **minor** versions roughly every three months and usually maintains **three minor** versions (the actual and the last two) with bug fixes and security updates. 
Patch releases are done more frequently.

Gardener needs to be configured accordingly to reflect the "logical state" of a version. 
It should be possible to define the Kubernetes or Machine image versions that still receive bug fixes and security patches and also vice-versa to define the version that are out-of-maintenance and are potentially vulnerable.
Moreover, this allows Gardener to "understand" the current state of a version and act upon it (more information in the following sections). 

Because the manual maintenance can be cumbersome, Gardener can automate certain activities 
involved in managing versions (see sections [Version Management](#version-management) and [Version Maintenance](#version-maintenance)).

## Overview

**As a Gardener operator**:
- I can classify a version based on it's logical state (`preview`, `supported`, `deprecated` and `expired` see [Version Classification](#version-classifications)).
- I can define which Machine image and Kubernetes versions are eligible for the auto update of Clusters during the maintenance time.
- I can optionally define how many minor versions (semVer) are to be considered "maintained".
- I can disallow the creation of clusters having a certain version (think of severe security issues).

**As an end-user/Shoot owner of Gardener**:
- I can get information which Kubernetes and Machine image versions are supported, in preview, deprecated or expired. 
- I can determine the time when my Shoot clusters Machine image and Kubernetes version will be forcefully updated to the next patch or minor version (in case the cluster is running a deprecated version with an expiration date).
- I can get this information via API from the CloudProfile.

## Version Classifications

In their lifetime, versions go through 4 distinct "logical states".
Versions usually start with the classification `preview`, then are promoted to `supported`, eventually `deprecated` and finally `expired`. 
This classification serves as a "point-of-reference" for end-users and also has implications during shoot creation and the maintenance time.

This information is programmatically available in the `CloudProfiles` of the Garden cluster. 

- **preview:** A `preview` version is a new version that has not yet undergone thorough testing, possibly a new release and needs time to be validated. There is a probability of unresolved issues and is therefore not yet recommended for production usage.
A Shoot does not update (neither `auto-update` or `force-update`) to  a `preview` version during the maintenance time. 
Also `preview` versions are not considered for the defaulting to the highest available version when deliberately omitting the patch version during Shoot creation.
Typically, after a fresh release of a new Kubernetes (e.g. v1.17.0) or Machine image version (e.g. coreos-2023.5) the operator tags it as `preview` until he has gained sufficient experience and regards this version to be reliable. 
After the operator gained sufficient trust, the version can be manually promoted to `supported`.  

- **supported:** A `supported` version is the recommended version for new and existing Shoot clusters. New Shoot clusters should use and existing clusters should update to this version.
Typically for Kubernetes versions, the latest Kubernetes patch versions of the actual (if not still in `preview`) and the last 3 minor Kubernetes versions are maintained by the community. An operator could define these versions as being `supported` (e.g. v1.16.1, v1.15.4, v1.14.9 and v1.13.12).

- **deprecated:** A `deprecated` version is a version that approaches the end of its lifecycle and can contain issues which are probably resolved in a supported version. 
New Shoots should not use this version any more. 
Existing Shoots will be updated to a newer version if `auto-update` is enabled (`.spec.maintenance.autoUpdate.kubernetesVersion` for Kubernetes version `auto-update` or `.spec.maintenance.autoUpdate.machineImageVersion` for machine machine image version `auto-update`).
Using automatic upgrades however does not guarantee that a Shoot runs a non-deprecated version, as the latest version (overall or of the minor version) can be deprecated as well.
Deprecated versions should have an expiration date set for eventual expiration. 

- **expired:** An `expired` versions has an expiration date in the past. 
 New clusters with that version cannot be created and existing clusters are forcefully migrated to a higher version during the maintenance time.
 
Below is an example how the relevant section of the CloudProfile might look like:
``` yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: alicloud
spec:
  kubernetes:
    versions:
      - classification: supported
        version: 1.17.1
      - classification: deprecated
        expirationDate: "2020-07-24T16:13:26Z"
        version: 1.17.0
      - classification: preview
        version: 1.16.6
      - classification: supported
        version: 1.16.5
      - classification: deprecated
        expirationDate: "2020-04-25T09:30:40Z"
        version: 1.16.4
      - classification: supported
        version: 1.15.7
      - classification: deprecated
        expirationDate: "2020-06-09T14:01:39Z"
        version: 1.15.6
```
 
 ## Version Requirements (Kubernetes and Machine image)
 
 The Gardener API server enforces the following requirements for versions:
  
 **Deletion a version** 
 - A version that is in use by a Shoot cannot be deleted from the CloudProfile.
 
 **Adding a version** 
 - A version must not have an expiration date in the past.
 - There can be only one `supported` version (there can be one supported version per Minor version if `Version Maintenance` is configured).
 
 ## Forceful migration of expired versions
 
 If a Shoot is running a version after its expiration date has passed, it will be forcefully migrated during it maintenance time.
 This happens **even if the owner has opted out of automatic cluster updates!**

For **Machine images**, the Shoots worker pools will be updated to the latest `non-preview` version of the pools respective image.

For **Kubernetes versions**, the forceful update picks the latest 'non-preview' patch version of the current minor version. 

 If the cluster is already on the latest patch version and the latest patch version is also expired, 
 it will continue with the latest patch version of the **next consecutive minor Kubernetes version**, so **it will result in an 
 update of a minor Kubernetes version!** 
 
 If that's expired as well, the update process repeats until a non-expired Kubernetes version is reached.
  
 **Depending on the circumstances described above, it can happen that the cluster receives multiple consecutive minor Kubernetes version updates!**
 As a Kubernetes version update cannot skip a minor version, the CloudProfile has to be maintained properly.

## Version Management

Using `Version Management` can help with managing the Kubernetes and Machine Image versions in the CloudProfile. It is not required and not enabled by default.

Characteristics of using `Version Management` are:
- Initial classification of versions in the CloudProfile having no classification.
- Enforces one `supported` version that has no expiration date
- Automatically deprecates versions with the correct `expiration date` (defined in `controllers.cloudProfile.kubernetesVersionManagement.expirationDuration` for Kubernetes versions and `controllers.cloudProfile.machineImageVersionManagement.expirationDuration` for machine images)
- Reconciliation when the CloudProfile has been updated

### Gardener Controller Manager configuration

The Gardener Controller Manager has to be configured to enable `Version Management`.
Below is an example of a config file enabling `Version Management` for Kubernetes as well as Machine Image versions. 
For each the configured expiration duration is 4 months. 
This means that versions that are being deprecated by the `Version Management` will expire at the date of deprecation + 4 months.

``` yaml
apiVersion: controllermanager.config.gardener.cloud/v1alpha1
kind: ControllerManagerConfiguration
controllers:
  cloudProfile:
    concurrentSyncs: 5
    kubernetesVersionManagement:
      expirationDuration: 2880h # defaults to 120 days / 4 months
    machineImageVersionManagement:
      expirationDuration: 2880h # defaults to 120 days / 4 months
```


## Version Maintenance

`Maintained` versions are commonly being referred to versions that have active development support for security updates and bug fixes. How many versions are being maintained is up to the individual project / product.
For instance, Kubernetes releases **minor** versions roughly every three months and usually maintains **three minor** versions (the actual and the last two). 
This means Kubernetes clusters having a version that is not `maintained` any more, run into the risk of having severe bugs or known security problems. 
Considering Kubernetes minor versions: 1.18 (being the most recent), 1.17, 1.16, 1.15 and 1.14. Then versions 1.18, 1.17, 1.16 are `maintained` and versions 1.15 and 1.14 are `unmaintained`

The operator of Gardener can optionally define `maintained` minor versions through the configuration file of the Gardener Controller Manager.
This can be done by specifying the number of maintained minor versions starting from the most recent. Given the Kubernetes project example above, the amount of maintained minor versions is `3`.
The configuration is located in the Gardener Controller Manager config at: `controllers.cloudProfile.kubernetesVersionManagement.versionMaintenance` and `controllers.cloudProfile.machineImageVersionManagement.versionMaintenance`.


`Version Maintenance` helps with the handling of `maintained` and `unmaintained` versions, specifically:
 - Enforces one `supported` version per `maintained` **minor version** (versus only one `supported` version without `Version Maintenance`)
   - be aware that if there is no supported version for a maintained minor version, it will pick the **latest** version independent of the classification and sets it to `supported`.
 - Deprecates all versions with an `unmaintained` minor (no matter what classification they have).
 - Sets the correct deprecation date for `unmaintained` versions (`controllers.cloudProfile.kubernetesVersionManagement.expirationDuration`).
 - Does not set an expiration date on the latest version with an `unmaintained` version. Otherwise, an undesired and automated forceful minor Kubernetes version upgrade is possible. This is potentially harmful for the deployed workload.  

Because Shoot clusters using an `unmaintained` minor version potentially have a higher security risk, Gardener will update these clusters after a certain date / grace-period.
By defining the field `expirationDurationUnmaintainedVersion` in the Gardener Controller Manager configuration ([see below](#gardener-controller-manager-configuration-for-version-maintenance)), Gardener can set a different expiration date for 
`unmaintained` versions than for `maintained` versions (the expiration date for `maintained` versions can be configured by the field `controllers.cloudProfile.kubernetesVersionManagement.expirationDuration`). 
For example `maintained` versions are deprecated with an expiration of four months, while `unmaintained` already expire in one. 
As described above, expired versions will be forcefully updated by Gardener.

### Gardener Controller Manager configuration for Version Maintenance

The Gardener Controller Manager has to be configured to enable the optional `Version Maintenance`.
This is an example Gardener Controller Manager configuration enabling `Version Maintenance` for Kubernetes, but not for Machine image versions:

``` yaml
apiVersion: controllermanager.config.gardener.cloud/v1alpha1
kind: ControllerManagerConfiguration
controllers:
  cloudProfile:
    concurrentSyncs: 5
    kubernetesVersionManagement:
      expirationDuration: 2880h # defaults to 120 days / 4 months
      versionMaintenance:
        maintainedKubernetesVersions: 3
        expirationDurationUnmaintainedVersion: 720h
    machineImageVersionManagement:
      expirationDuration: 2880h # defaults to 120 days / 4 months
```

## Duties of the Gardener Operator

Even though Gardener strives for high automation, we think that it leads to safer operations 
having a human operator in charge of the following operations: 

- Adding `preview` versions
- Promotion of a `preview` version to `supported`. We think this should be done on a case-by-case basis, hence Gardener deliberately does not offer an automation in this case.
- Deprecate a `supported` version
   - This is of particular importance when using `Version Management`. As it defines an initial classification for versions with no classification, 
 it can happen that it tries to introduce another `supported` version even though there is already one present. That is forbidden and blocked by the Gardener API server. 
 That situation occurs when specifying an unclassified version that is the overall latest version (or the latest version of a minor version when using `Version Maintenance`).
 It is the operator duty to make sure that this does not happen. Specifically when introducing a new latest version without a classification, the current `supported` version should be manually deprecated ahead of time.
- Deletion of versions. Before deletion, the operator has to make sure the version is not in use by any Shoot. The recommended way to achieve this, is to deprecate the version and set an expiration date. 
Clusters with `auto-update` configured will automatically update during their next maintenance. The remainder is forcefully updated during maintenance time after the version is finally expired.
- Setting an `expiration date` on the latest patch version of an `unmaintained`  Kubernetes minor version. This is to prevent that Gardener automatically forces minor version upgrades for Clusters.
    - After the deprecation date passed, it will force a Kubernetes minor version update during the next maintenance time window of the Shoot.

## Common scenarios for the Gardener Operator

The following describes common scenarios the Gardener operator regularly faces when managing the Kubernetes and Machine image versions of a Gardener installation.
We assume a CloudProfile that looks schematically like this:

``` yaml
1.18.4 :  preview
1.18.3 : (VersionMaintenance: supported, otherwise: deprecated)
1.17.0 : (VersionMaintenance: supported, otherwise: deprecated)
1.16.0 : (VersionMaintenance: supported, otherwise: deprecated)
1.15.1 : deprecated with no expiration date
1.15.0 : deprecated with expiration date that is not expired
1.14.0 : deprecated with expiration date already expired
```
For `Version Maintenance`, the Gardener Controller Manager configuration in `controllers.cloudProfile.kubernetesVersionManagement.versionMaintenance` is set to `3`.

**Scenarios**

* * *
***A new patch version has been released and should be added to the CloudProfile.***

**Example**: Kubernetes releases version 1.18.4. 
 
 -  Add the version and set the classification to `preview`. There can be multiple preview versions.
 
**Example 2**: Kubernetes releases version 1.17.1. 
 
 - With `Version Management` enabled: Version 1.17.1 will be deprecated wit an expiration date right away.
 
 - With `Version Maintenance` enabled: Version 1.17.1 has a maintained minor version. 
    If it is classified as `preview`, it stays unchanged. If added without a classification, version 1.17.1 will be classified as `supported` and 
    version 1.17.0 will deprecated with the expiration date for `supported` versions. 

* * *
***A new minor release version has been released and should be added to the CloudProfile.***

**Example**: Kubernetes releases version 1.19.0.

 - Deprecate the current `supported` version, optionally with an expiration date. Afterwards add the new version as a `preview` version.
 
 - `Version Management` enabled: Add version 1.19.0 without a classification. 1.19.0 will be classified as `supported` and version 1.18.3 will be expired with the configured expiration date.
 
 - With `Version Maintenance` enabled: Add version 1.19.0 without a classification. `Version Maintenance` makes sure that versions now having an `unmaintained` minor, will be accordingly deprecated. 1.19.0 will be classified as `supported` and version 1.16.0 will be deprecated having no expiration date.
 
* * *
***A `preview` version should be promoted to `supported`.***

**Example**: Kubernetes version 1.18.4 should be promoted to `supported`.

 - Deprecate the currently `supported` version 1.18.3, optionally with a expiration date. Afterwards change the classification of the version 1.18.4 from `preview` to `supported`.
 
 - `Version Management` enabled: Deprecate the current `supported` version 1.18.3. An expiration date is being set automatically. Afterwards change the classification of the version 1.18.4 from `preview` to `supported`
 
 - With `Version Maintenance` enabled: Same as for `Version Management`, but make sure to deprecate the `supported` version of the particular minor version (in this example: 1.18.3).
 
* * *
***The `supported` version should be deprecated.***

**Example**: Version 1.18.3 should be deprecated.

 - Deprecate version 1.18.3 by setting the classification `deprecated`, optionally with a expiration date.
 
 - `Version Management` enabled: By just adding a new version e.g 1.18.4 without a classification, Version 1.18.3 will be deprecated and 1.18.4 becomes the new `supported` version.  
   However, is there no higher non-preview version available, version 1.18.3 will be reset to `supported` without an expiration date. Having one `supported` version is enforced by the `Version Management`.
 
 - With `Version Maintenance` enabled:  Similar as for `Version Management`. Having one `supported` version per `maintained` minor version is enforced. 
 
* * *
**The current `supported` version has problems and should be replaced with the previous `supported` version.**

**Example**: Version 1.18.3 has problems.

a) Version is not in use by any Shoot.
 - delete the version from the CloudProfile. 
 - Manually change the classification of the previously `supported` version.
 - `Version Management` and `Version Maintenance`: nothing else has to be done. 

b) Shoots are already running the erroneous version
 -  Make sure there is a higher non-preview version available in the CloudProfile.
 -  Manually change the classification of the erroneous version to `deprecated` with an `expiration date` in the past. Change the version to `supported` again.
 - `Version Management` and `Version Maintenance`: The current `supported` version needs to be labeled as `preview` with an `expiration date` in the past. 
    Because one supported version is enforced, the latest `deprecated` version will automatically be classified as `supported` again.
    Shoots running the `preview` version with an `expiration date` in the past will be forcefully updated. 
    

c) Shoots are already running the erroneous version and it is the overall latest version:
 - Shoots cannot upgrade to a newer version as there is none available. Also a downgrade is not supported. 
   To reduce the blast-radius, label the version as `deprecated` with an `expiration date` in the past to prevent `auto-updates`, defaulting and new cluster creation with that version.
   The operator has to wait for a newer patch version or delete the Shoot clusters in question. Only then, the version can be deleted from the CloudProfile.
 - `Version Management` and `Version Maintenance`: same approach, but label the version as `preview`, otherwise the automation will reconcile it to be the `supported` version.
 
* * *
***A version should be removed from the CloudProfile, is already expired but cannot be deleted.***

**Example**: Clusters are using version 1.14.0. 
 
 - Version 1.14.0 can only be removed it is not in use by any Shoot anymore.
 - Wait 24 hours to make sure that the maintenance of all Shoots has run - this should have forceully upgraded the clusters.
 - Make sure the maintenance was successful (check the events on the Shoot). A misconfigured CloudProfile can block the maintenance operations.
 - Make sure that no Shoot 'stuck in deletion' is still using version 1.14.0. Maintenance is not being performed for clusters that are being deleted. At the moment it is still required to successfully delete the shoot before the version can be removed.