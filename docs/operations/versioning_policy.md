# Gardener Versioning Policy

## Goal

- As a Garden operator I would like to define a clear Kubernetes version policy, which informs my users about deprecated or expired Kubernetes versions.
- As an user of Gardener, I would like to get information which Kubernetes version is supported or in preview. I want to be able to get this information via API (cloudprofile).

## Motivation

The Gardener Versioning Policy makes it possible for operators to classify Kubernetes and Machine image versions, while they are going through their "maintenance life-cycle".

The Kubernetes community releases **minor** versions roughly every three months and usually maintains **three minor** versions (the actual and the last two) with bug fixes and security updates. 
Patch releases are done more frequently.
Operators of Gardener are able to define their own Kubernetes version policy. 

## Version Classifications

This is a specification for Kubernetes and machine image versions.
An operator is able to classify versions while they go through their "maintenance life-cycle". 
In their lifetime, versions go through 4 distinct "logical states".

Versions start with the classification **preview**, then are promoted to **supported**, eventually **deprecated**, and finally **expired**. 

This classification serves as a "point-of-reference" for end-users and also has implications during shoot creation and the maintenance time.

This information is programmatically available in the `CloudProfiles` of the Garden cluster. 

- **preview:** A 'Preview' version is a new version that has not yet undergone thorough testing, possibly a new release and needs time to be validated. There is a probability of unresolved issues and is therefore not yet recommended for production usage.
A Shoot does not update (neither auto Update or force update) to  a _preview_ version during the maintenance time. 
Also _preview_ versions are not considered for the defaulting to the highest available version when deliberately omitting the patch version during Shoot creation.
Typically, after a fresh release of a new Kubernetes (e.g. v1.17.0) or Machine image version (e.g. coreos-2023.5) the operator tags it as _preview_ until he has gained sufficient experience and regards this version to be reliable. 
After the operator gained sufficient trust, the version can be manually promoted to 'supported'.  

- **supported:** A 'Supported' version is the recommended version for new and existing Shoot clusters. New Shoot clusters should use and existing clusters should update to this version.
Typically for Kubernetes versions, the latest Kubernetes patch versions of the actual (if not still in _preview_) and the last 3 minor Kubernetes versions are maintained by the community. An operator could define these versions as being _supported_ (e.g. v1.16.1, v1.15.4, v1.14.9 and v1.13.12).

- **deprecated:** A 'Deprecated' version is a version that approaches the end of its lifecycle and can contain issues which are probably resolved in a supported version. 
New Shoots should not use this version any more. 
Existing Shoots will be updated to a newer version if auto update is enabled (`.spec.maintenance.autoUpdate.kubernetesVersion` for Kubernetes version auto update or `.spec.maintenance.autoUpdate.machineImageVersion` for machine machine image version auto update).
Using auto update however does not guarantee that a Shoot runs a non deprecated version, as the latest version can be deprecated as well (for Kubernetes versions: the latest patch version of a particular minor version).
Deprecated versions should have an expiration date set for eventual expiration 
(also see [Duties of the Gardener operator](#duties-of-the-gardener-operator) on how to handle expiration days on `unmaintained` Kubernetes versions). 

- **expired:** An 'Expired' versions has an expiration date in the past. 
 New clusters with that version cannot be created and existing clusters are forcefully migrated in their maintenance time to a higher version.
 
 ## Forceful migration of expired versions
 
 If a Shoot is running a version after its expiration date has passed, it will be forcefully migrated during it maintenance time.
 This happens **even if the owner has opted out of automatic cluster updates!**

For **Machine images**, the Shoots worker pools will be updated to the latest 'non-preview' version of the pools respective image.

For **Kubernetes versions**, the forceful update picks the latest 'non-preview' patch version of the current minor version. 

 If the cluster is already on the latest patch version and the latest patch version is also expired, 
 it will continue with the latest patch version of the **next consecutive minor Kubernetes version**, so **it will result in an 
 update of a minor Kubernetes version!** 
 
 If that's expired as well, the update process repeats until a non-expired Kubernetes version is reached.
  
 **Depending on the circumstances described above, it can happen that the cluster receives multiple consecutive minor Kubernetes version updates!**.
 As a Kubernetes version update cannot skip a minor version, the CloudProfile has to be maintained properly.

##Duties of the Gardener Operator

Even though we strive for high automation, we think that it leads to safer operations 
having a human operator in charge of the following operations: 

- Promotion of a `preview` version to `supported`
- Deletion of versions (cannot be in use by any shoot any more and need to have an expiration date in the past)
- Setting an `expiration date` on the latest patch version of an 'unmaintained' Kubernetes minor version
    - After the deprecation date passed, it will force a kubernetes minor version update during the next maintenance time window of the Shoot.

## Version Requirements

The Gardener API server enforces the following requirements:

### Kubernetes Versions 
**Deletion of a Kubernetes Version** 
- A Kubernetes version that is in use by a Shoot cannot be deleted 

**Adding a Kubernetes Version** 
- A version must not have an expiration date in the past
- A 'preview' version must be the latest (semVer) patch version of that particular minor version
- A 'supported' version has to be lower (semVer) than all preview versions

### Machine image Versions 
**Deletion of a Machine image version** 
- A machine image version that is in use by a worker pool of a Shoot cannot be deleted

**Adding a Machine image Version** 
- A version must not have an expiration date in the past
- A 'preview' version must be the latest (semVer) patch version of that particular minor version
- A 'supported' version has to be lower (semVer) than all preview versions

# Automatic Version Management 

Gardener can manage the Kubernetes and the Machine image versions in the CloudProfile.
This comes with the following benefits:

- central configuration of expiration dates for Kubernetes and Machine image versions. 
Automatically sets expiration dates for versions based on the configuration in the Gardener Controller Manager. 
    -  KubernetesVersionManagement: expiration duration for maintained and unmaintained versions
    -  MachineImageVersionManagement: same expiration duration for all versions 

- defines 'maintained' Kubernetes versions. These are versions that receive bug fixes and maintenance by the community. 
 Makes sure that only 'maintained' versions are classified as 'preview' or 'supported'.
 This can be configured with the field `maintainedKubernetesVersions` in the `kubernetesVersionManagement`.
 
- compute and set classifications for versions having currently no classification

- compute and set classifications when the CloudProfile changes - e.g when versions have been added.

- enables promoting a 'preview' version to 'supported' without manually having to deprecate the currently 
'supported' version with the right expiration date. Makes sure there is only one 'supported version'.

Automatic Version Management can be configured in the Gardener Controller Manager's `ControllerManagerConfiguration` 
of the CloudProfile controller and can be enabled via the flag `controllers.cloudProfile.kubernetesVersionManagement.enabled`  
for Kubernetes versions and `controllers.cloudProfile.machineImageVersionManagement.enabled` for machine image versions.

Example Gardener Controller Manager config file

``` yaml
controllers:
  cloudProfile:
    # monitorPeriod: 40s
    concurrentSyncs: 5
    kubernetesVersionManagement:
      enabled: true # defaults to false
#      maintainedKubernetesVersions: 4 # defaults to 3
#      expirationDurationMaintainedVersion: 2880h # defaults to 120 days / 4 months
#      expirationDurationUnmaintainedVersion: 720h # defaults to 30 days / 1 months
    machineImageVersionManagement:
      enabled: true # defaults to false
#      expirationDuration: 2880h # defaults to 120 days / 4 months

```