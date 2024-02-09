---
title: Private Cloud Profiles
gep-number: 25
creation-date: 2024-02-08
status: implementable
authors:
- "@benedictweis"
- "@timuthy"
reviewers:
- "@rfranzke"
- "@ScheererJ"
---

# GEP-25: Private Cloud Profiles

## Table of Contents

- [GEP-25: Private Cloud Profiles](#gep-25-private-cloud-profiles)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Context](#context)
    - [Current State](#current-state)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [Approach](#approach)
    - [Manifest](#manifest)
    - [Rendering](#rendering)
    - [Custom RBAC verb](#custom-rbac-verb)
    - [Automated removal of outdated Kubernetes versions and machine image versions](#automated-removal-of-outdated-kubernetes-versions-and-machine-image-versions)
    - [Cross-project sharing](#cross-project-sharing)
    - [Multi-Level-Inheritance](#multi-level-inheritance)
  - [Alternatives](#alternatives)
    - [Arbitrary Value Fields](#arbitrary-value-fields)
    - [Private Cloud Profiles by Selection](#private-cloud-profiles-by-selection)

## Summary

Cloud profiles are currently non-namespaced objects that are configured centrally and consumed by any end user creating a shoot in the landscape. However, some projects require lots of special entries in a cloud profile which adds unnecessary bulk to the cloud profile, as there are lots of entries in it, that are only relevant to one project. Additionally, is usually updated in a certain time interval which means that might projects have to wait several days or even weeks before they can use the changed cloud profile.

This GEP proposes a mechanism that will allow project administrators to create private cloud profiles that are only visible to and can only be consumed by the project they were created in.

## Motivation

### Context

Cloud profiles are an integral component of Gardener for managing shoot resources. They are currently managed centrally by Gardener and can be consumed by any Gardener user. However, some teams require frequent changes to a cloud profile. There are a few reasons why:

1. Testing with different machine types than the ones present in the cloud profile.
2. Needing to use different volume types than the ones present in the cloud profile.
3. Extending the expiration date of Kubernetes versions. Teams might need to hang on to an older version of Kubernetes as they might not be ready for an upgrade as of the set expiration date. Given that the cloud profile is a cluster-scoped resource, it is currently not possible to extend the expiration date for shoots in one project but only centrally for all projects.
4. Extending the expiration date for machine images. For the same reasons as extending the expiration date of the Kubernetes versions.
5. Some gardener users might have access to custom cloud provider regions that others do not have access to, they currently have no way of using custom cloud provider regions.

### Current State

Cloud Profiles are non-namespaced resources. This means that in a typical gardener installation, only a handful of cloud profiles (i.e. one per cloud provider) exist. They can be consumed by any project's clusters. Consequently, when a project requires changes to cloud profiles for any of the reasons mentioned in the [context section](#context), they are changed for the entire gardener landscape. Since some projects might require frequent changes to a cloud profile, this become quite a cumbersome process. Additionally, it adds confusion to every project's workflow if say for example a team requires an extension of the expiration date of a Kubernetes version since they will be wondering why it was extended.

### Goals

- Reduce load on Gardener operators for maintaining cloud profiles
- Possibly reduce the time a team has to wait until a change in a cloud profile is reflected in production
- Make it so that project-scoped information in the cloud profile is only visible to the relevant project
- Full backward compatibility

### Non-Goals

- Automate approval process for changes in Gardeners cloud profiles
- Add different, possibly unsupported Kubernetes versions or machine image versions and names

## Proposal

It is proposed to implement a solution that enables project administrators to create custom (private) cloud profiles. These cloud profiles would be consumed by project users of the project they were created in.

### Approach

First of all, a new, namespaced API object is defined, the `PrivateCloudProfile`. Its type definition is very similar to the `CloudProfile` object but it omits the `type`, `providerConfig` and `seedSelector` fields. The `type` field is omitted as it is inherited from the parent cloud profile. The `providerConfig` field is omitted as gardener itself does not know anything about the structure of this field and cannot merge it accordingly. The `seedSelector` field is omitted as it is too complex for this GEP but may be added in the future.

The general approach is that a private cloud profile inherits from a cloud profile using a `parent` field in the private cloud profile and should therefore not change the type or provider config. Fields such as `machineTypes`, `volumeTypes`, `regions` and `caBundle` are going to be merged with the parent cloud profile. However, a restriction need to be defined so that the `kubernetes` and `machineImages` fields in a private cloud profile may only be adjusted by a gardener operator to reduce the chance of a team staying on an unsupported kubernetes version. A similar problem is already solved in Gardener using custom RBAC verbs [here](https://github.com/gardener/gardener/blob/master/plugin/pkg/global/customverbauthorizer), the approach is described in more detail in the [custom RBAC verb section](#custom-rbac-verb).

Currently, a shoot's reference to a cloud profile is immutable. When enabling private cloud profiles, it is necessary to allow a change to the cloud profile reference. Although it is very important to only allow a change to a private cloud profile that uses the shoot's current cloud profile as a parent. Additionally, the change may go in the other direction, changing from a private cloud profile to the private cloud profile's parent cloud profile.

### Manifest

A private cloud profile could look like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: PrivateCloudProfile
metadata:
  name: private-cloud-profile-xyz
  namespace: project-xyz
spec:
  parent: aws-central-cloud-profile
  kubernetes:
    versions:
      - version: 1.28.6
        expirationDate: 2024-06-06T01:02:03Z
  machineImages:
    - name: suse-chost
      versions:
        - version: 16.4
          expirationDate: 2023-08-8T23:59:59Z
  machineTypes: 
    - name: m5.xlarge
      cpu: "8"
      gpu: "0"
      memory: 16Gi
  volumeTypes:
    - name: ab6
      class: premium
      usable: true
  regions:
    - name: europe-special
      zones:
        - name: europe-custom-1a
        - name: europe-custom-1b
        - name: europe-custom-1c
```

### Rendering

Since Gardener is designed around using the `CloudProfile` object for managing infrastructure details for a shoot, the private cloud profile has to be rendered so that a `CloudProfile` object is emitted. This rendered cloud profile will be written to the status of the `PrivateCloudProfile` object.

Suppose we have a simplified cloud profile that looks like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: aws-central-cloud-profile
spec:
  type: aws
  kubernetes:
    versions:
      - version: 1.27.1
      - version: 1.26.3
      - version: 1.25.8
      - version: 1.24.6
      - version: 1.28.6
        expirationDate: 2023-02-02T01:02:03Z
  machineImages:
    - name: suse-chost
      versions:
        - version: 15.4
        - version: 14.4
        - version: 13.6
  machineTypes: 
    - name: m5.large
      cpu: "4"
      gpu: "0"
      memory: 8Gi
  volumeTypes:
    - name: gp3
      class: standard
      usable: true
  regions:
    - name: europe-central-1
      zones:
        - name: europe-central-1a
        - name: europe-central-1b
        - name: europe-central-1c
```

and a private cloud profile from the [manifest section](#manifest).

After the rendering is done, the private cloud profile will look like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: PrivateCloudProfile
metadata:
  name: private-cloud-profile-xyz
  namespace: project-xyz
spec:
  parent: aws-central-cloud-profile
  kubernetes:
    versions:
      - version: 1.28.6
        expirationDate: 2024-06-06T01:02:03Z
  machineImages:
    - name: suse-chost
      versions:
        - version: 16.4
          expirationDate: 2023-08-8T23:59:59Z
  machineTypes: 
    - name: m5.xlarge
      cpu: "8"
      gpu: "0"
      memory: 16Gi
  volumeTypes:
    - name: ab6
      class: premium
      usable: true
  regions:
    - name: europe-special
      zones:
        - name: europe-custom-1a
        - name: europe-custom-1b
        - name: europe-custom-1c
status:
  cloudProfile:
    apiVersion: core.gardener.cloud/v1beta1
    kind: CloudProfile
    metadata:
      name: private-cloud-profile-xyz
    spec:
      type: aws
      kubernetes:
        versions:
          - version: 1.27.1
          - version: 1.26.3
          - version: 1.25.8
          - version: 1.24.6
          - version: 1.28.6
            expirationDate: 2024-06-06T01:02:03Z
      machineImages:
        - name: suse-chost
          versions:
            - version: 16.4
              expirationDate: 2023-08-8T23:59:59Z
            - version: 15.4
            - version: 14.4
            - version: 13.6
      machineTypes: 
        - name: m5.large
          cpu: "4"
          gpu: "0"
          memory: 8Gi
        - name: m5.xlarge
          cpu: "8"
          gpu: "0"
          memory: 16Gi
      volumeTypes:
        - name: ab6
          class: premium
          usable: true
        - name: gp3
          class: standard
          usable: true
      regions:
        - name: europe-central-1
          zones:
            - name: europe-central-1a
            - name: europe-central-1b
            - name: europe-central-1c
        - name: europe-special
          zones:
            - name: europe-custom-1a
            - name: europe-custom-1b
            - name: europe-custom-1c
```

The rendering is done by a new custom controller registered to the `gardener-controller-manager`. If a merge conflict is to appear during the rendering process, it should be noted somewhere in the status and the parent cloud profile should be preferred.

### Custom RBAC verb

To prevent users from entering arbitrary values in the `kubernetes` and `machineImages` fields, two custom RBAC verbs may be introduced. It can then be checked if the user that is creating or updating the `kubernetes` or `machineImages` field is authorized to do so.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: privatecloudprofile-modify-special-fields
rules:
  - apiGroups: [""]
    resources: ["privatecloudprofile"]
    verbs: ["modify-spec-kubernetes", "modify-spec-machineImages"]
```

### Automated removal of outdated Kubernetes versions and machine image versions

Expired Kubernetes versions in cloud profiles are currently being removed manually. This is not an issue since there is a very limited amount of cloud profiles. However, if any project administrator is allowed to create a private cloud profile that can then have its Kubernetes versions expiration dates extended, it cloud become a very cumbersome process to manually remove all the expired Kubernetes versions. It is therefore proposed to implement a custom controller managed by `gardener-controller-manager`, that reconciles private cloud profiles (and possibly even regular cloud profiles) and removes any Kubernetes versions that are past their expiration date. The same could be done for machine image versions.

However, a problem arises as the Kubernetes/machine image versions are referenced in the `providerConfig` and would therefore need to be removed manually anyway.

### Cross-project sharing

A use case could be defined where a private cloud profile might want to be shared across multiple projects and not just be used within the project it was created id. However, this use case seems to be very slim so this functionality will not be implemented as of now. However, when taking a broader view of Private Seeds and Cloud in Country, this feature is going to be necessary at some point. Still, it is not planned to be implemented as of this GEP.

### Multi-Level-Inheritance

Theoretically, a private cloud profile could inherit from a cloud profile that already inherits from a cloud profile. However, this should probably not be allowed since it presents some major challenges.

1. The rendering process now has to merge multiple instead of two objects
2. The parent field becomes more complex because it could point to a cloud profile or to a private cloud profile.

Because of this, multi-inheritance will not be supported as of this GEP.

## Alternatives

### Arbitrary Value Fields

Instead of specifying a private cloud profile resource, an end user could be allowed to enter arbitrary names in fields such as `machineTypes`. The entry would then, if the user enters a wrong value, throw an error when provisioning the resource at the cloud provider. However, this approach does not seem feasible as the metadata that is specified in a cloud profile like the number of CPUs and amount of RAM is used in the trial clusters to validate quotas. Therefore, an exception would need to be developed for this specific, and possibly other, use cases.

### Private Cloud Profiles by Selection

Instead of using a single inheritance-based approach with the `parent` field, a similar approach as [aggregated cluster roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles) in Kubernetes could be used. In this approach, cloud profiles would be defined without a parent and would be aggregated together. However, this approach is not well suited as it can not clearly define which cloud profile overwrites which fields. A restriction could be defined that when combining multiple cloud profiles, no merge conflicts must be introduced, making the approach more reasonable.
