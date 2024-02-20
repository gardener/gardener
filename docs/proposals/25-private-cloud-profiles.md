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
    - [Multi-Level inheritance](#multi-level-inheritance)
  - [Alternatives](#alternatives)
    - [Arbitrary Value Fields](#arbitrary-value-fields)
    - [Private Cloud Profiles by Selection](#private-cloud-profiles-by-selection)

## Summary

[CloudProfiles](https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md#cloudprofiles) are non-namespaced objects that are managed centrally by Gardener operators. They usually don't only contain global configuration, but options that are relevant to certain projects only (e.g. special machine types). This increases the operation burden for operators and clutters `CloudProfile` objects. On the other hand, users are blocked until the requested special configuration is rolled out to the desired landscapes.

This GEP proposes a mechanism that allows project administrators to create `PrivateCloudProfile`s that are only visible in the project they belong to.

## Motivation

### Context

`CloudProfile`s are an integral component of Gardener for managing shoot clusters. They are currently managed centrally by Gardener operators and can be consumed by any Gardener user. However, some teams require frequent changes to a `CloudProfile`, mainly for the following reasons:

1. Testing with different machine types than the ones present in the `CloudProfile`.
2. Need to use different volume types than the ones present in the `CloudProfile`.
3. Extending the expiration date of Kubernetes versions. Given that the `CloudProfile` is a cluster-scoped resource, it is currently not possible to extend the expiration date for shoots in one project but only centrally for all projects.
4. Extending the expiration date for machine images. For the same reasons as extending the expiration date of the Kubernetes versions.
5. Some gardener users might have access to custom cloud provider regions that others do not have access to. They currently have no way of using custom cloud provider regions.

### Current State

`CloudProfile`s are non-namespaced resources. This means that in a typical Gardener installation, only a handful of `CloudProfile`s (typically one per cloud provider) exist. They can be consumed by any shoot cluster. Consequently, when a project requires changes to `CloudProfile`s for any of the reasons mentioned in the [context section](#context), they are changed for the entire Gardener landscape. Since some projects might require frequent changes, this becomes quite a cumbersome process on both, the operators' and users' sides.

### Goals

- Reduce load on Gardener operators for maintaining `CloudProfile`s
- Possibly reduce the time a team has to wait until a change in a `CloudProfile` is reflected in the landscape.
- Make it so that project-scoped information in the `CloudProfile` is only visible to the relevant project
- Full backward compatibility

### Non-Goals

- Automate the approval process for changes in `CloudProfile`s
- Add different, possibly unsupported Kubernetes versions or machine image versions and names

## Proposal

It is proposed to implement a solution that enables project administrators to create custom (private) `CloudProfile`s. These `CloudProfile`s would be consumed by project users of the project they were created in.

### Approach

First of all, a new, namespaced API object `PrivateCloudProfile` is defined. Its type definition is very similar to the `CloudProfile` object, but omits the following fields:

The general approach is that a `PrivateCloudProfile` inherits from a `CloudProfile` using a `parent` field. Fields such as `machineTypes`, `volumeTypes`, `regions` and `caBundle` are going to be merged with the parent `CloudProfile`. However, a restriction needs to be defined so that the `kubernetes` and `machineImages` fields in a `PrivateCloudProfile` may only be adjusted by a Gardener operator to reduce the chance of a team staying on an unsupported Kubernetes version. A similar problem is already solved in Gardener using custom RBAC verbs [here](https://github.com/gardener/gardener/blob/master/plugin/pkg/global/customverbauthorizer), see [custom RBAC verb section](#custom-rbac-verb) for more information.

Currently, the shoot's reference to a `CloudProfile` is immutable. This validation will be relaxed to allow updating to a `PrivateCloudProfile` whose parent is the same as the currently configured `CloudProfile`. The change will also be reversible, i.e. switching from `PrivateCloudProfile` to `CloudProfile`.

### Manifest

A `PrivateCloudProfile` could look like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: PrivateCloudProfile
metadata:
  name: aws-profile-xyz
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

Since Gardener is designed around using the `CloudProfile` object for managing infrastructure details for a shoot, the `PrivateCloudProfile` has to be rendered so that a `CloudProfile` object is emitted. This rendered `CloudProfile` will be written to the status of the `PrivateCloudProfile` object.

Suppose we have a simplified `CloudProfile` that looks like this:

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

and a `PrivateCloudProfile` from the [manifest section](#manifest).

After the rendering is done, the `PrivateCloudProfile` will look like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: PrivateCloudProfile
metadata:
  name:  aws-profile-xyz
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

The rendering is done by a new custom controller registered to the `gardener-controller-manager`. Merge conflicts can not arise during the merge process as they are caught by static validation and an admission plugin for validating the `PrivateCloudProfile` object.

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

Expired Kubernetes versions in `CloudProfile`s are currently being removed manually. This is not an issue since there is a very limited amount of `CloudProfile`s. However, if any project administrator is allowed to create a `PrivateCloudProfile` that can then have its Kubernetes versions expiration dates extended, it could become a very cumbersome process to manually remove all the expired Kubernetes versions. One possible solution is to implement a custom controller managed by `gardener-controller-manager`, that reconciles `PrivateCloudProfile`s (and possibly even regular `CloudProfile`s) and removes any Kubernetes versions that are past their expiration date. The same could be done for machine image versions.

However, a problem arises as the Kubernetes/machine image versions are referenced in the `providerConfig` and would therefore need to be removed manually anyway. Because of that, this feature is not part of this GEP.

### Cross-project sharing

A use case could be defined where a `PrivateCloudProfile` might want to be shared across multiple projects and not just be used within the project it was created in. However, this use case seems to be very slim so this functionality will not be implemented as of now. However, when taking a broader view of Private Seeds and Cloud in Country, this feature is going to be necessary at some point. Still, it is not planned to be implemented as part of this GEP.

### Multi-Level inheritance

Theoretically, a `PrivateCloudProfile` could inherit from a `CloudProfile` that already inherits from a `CloudProfile`. However, this should probably not be allowed since it presents some major challenges.

1. The rendering process would need to be recursive
2. The `parent` field becomes more complex because it could point to a `CloudProfile` or a `PrivateCloudProfile`

Because of this, multi-level inheritance will not be implemented as of this GEP.

## Alternatives

### Arbitrary Value Fields

Instead of specifying a `PrivateCloudProfile` resource, an end user could be allowed to enter arbitrary names in fields such as `machineTypes`. The entry would then, if the user enters a wrong value, throw an error when provisioning the resource at the cloud provider. However, this approach does not seem feasible as the metadata that is specified in a `CloudProfile` like the number of CPUs and amount of RAM is used in the trial clusters (see [Shoot Qutas](https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md#shoot-quotas)) to validate quotas and to enable the scale-from-zero feature. Therefore, an exception would need to be developed for this specific, and possibly other, use cases.

### Private Cloud Profiles by Selection

Instead of using a single inheritance-based approach with the `parent` field, a similar approach as [aggregated cluster roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles) in Kubernetes could be used. In this approach, `CloudProfile`s would be defined without a parent and would be aggregated together. However, this approach is not well suited as it can not clearly define which `CloudProfile` overwrites which fields. A restriction could be defined that when combining multiple `CloudProfile`s, no merge conflicts must be introduced, making the approach more reasonable.
