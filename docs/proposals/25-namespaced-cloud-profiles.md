---
title: Namespaced Cloud Profiles
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

# GEP-25: Namespaced Cloud Profiles

## Table of Contents

- [GEP-25: Namespaced Cloud Profiles](#gep-25-namespaced-cloud-profiles)
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
    - [Adjusting `Shoot`s `cloudProfileName` field](#adjusting-shoots-cloudprofilename-field)
    - [Migration path](#migration-path)
  - [Outlook](#outlook)
    - [Cross-project sharing](#cross-project-sharing)
    - [Multi-Level inheritance](#multi-level-inheritance)
    - [`NamespacedCloudProfile`s replacing regular `CloudProfile`s](#namespacedcloudprofiles-replacing-regular-cloudprofiles)
  - [Alternatives](#alternatives)
    - [Arbitrary Value Fields](#arbitrary-value-fields)
    - [Namespaced Cloud Profiles by Selection](#namespaced-cloud-profiles-by-selection)

## Summary

[CloudProfiles](https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md#cloudprofiles) are non-namespaced objects that are managed centrally by Gardener operators. They usually contain not only global configuration, but options that are relevant to certain projects only (e.g. special machine types). This increases the operation burden for operators and clutters `CloudProfile` objects. On the other hand, users are blocked until the requested special configuration is rolled out to the desired landscapes.

This GEP proposes a mechanism that allows project administrators to create `NamespacedCloudProfile`s that are only visible in the project they belong to.

## Motivation

### Context

`CloudProfile`s are an integral component of Gardener for managing shoot clusters. They are currently managed centrally by Gardener operators and can be consumed by any Gardener user. However, some teams require frequent changes to a `CloudProfile`, mainly for the following reasons:

1. Testing with different machine types than the ones present in the `CloudProfile`.
2. Need to use different volume types than the ones present in the `CloudProfile`.
3. Extending the expiration date of Kubernetes versions. Given that the `CloudProfile` is a cluster-scoped resource, it is currently not possible to extend the expiration date for shoots in one project but only centrally for all projects.
4. Extending the expiration date for machine images. For the same reasons as extending the expiration date of the Kubernetes versions.

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

It is proposed to implement a solution that enables project administrators to create custom (namespaced) `CloudProfile`s. These `CloudProfile`s would be consumed by users of the project they were created in.

### Approach

First of all, a new, namespaced API object `NamespacedCloudProfile` is defined. Its type definition is very similar to the `CloudProfile` object.

The general approach is that a `NamespacedCloudProfile` inherits from a `CloudProfile` using a `parent` field. Fields such as `machineTypes`, `volumeTypes` and `caBundle` are going to be merged with the parent `CloudProfile`. However, a restriction needs to be defined so that the `kubernetes` and `machineImages` fields in a `NamespacedCloudProfile` may only be adjusted by a Gardener operator to reduce the chance of a team staying on an unsupported Kubernetes version. A similar problem is already solved in Gardener using custom RBAC verbs [here](https://github.com/gardener/gardener/blob/master/plugin/pkg/global/customverbauthorizer), see [custom RBAC verb section](#custom-rbac-verb) for more information.

Currently, the shoot's reference to a `CloudProfile` is immutable. This validation will be relaxed to allow updating to a `NamespacedCloudProfile` whose parent is the same as the currently configured `CloudProfile`. The change will also be reversible, i.e. switching from `NamespacedCloudProfile` to `CloudProfile`.

The `NamespacedCloudProfile` will not include the `providerConfig` and `regions` fields. The contents of the `providerConfig` field are not known to Gardener but only to the provider extensions and can therefore not be merged without consulting the appropriate extension. The `regions` field typically needs some kind of entry in the `providerConfig` and is therefore excluded as well.

### Manifest

A `NamespacedCloudProfile` could look like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: NamespacedCloudProfile
metadata:
  name: aws-profile-xyz
  namespace: project-xyz
spec:
  parent:
    kind: CloudProfile
    name: aws-central-cloud-profile
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
```

### Rendering

Since Gardener is designed around using the `CloudProfile` object for managing infrastructure details for a shoot, the `NamespacedCloudProfile` has to be rendered so that a `CloudProfile` object is emitted. This rendered `CloudProfile` will be written to the status of the `NamespacedCloudProfile` object.

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
```

and the `NamespacedCloudProfile` from the [manifest section](#manifest).

After the rendering is done, the `NamespacedCloudProfile` will look like this:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: NamespacedCloudProfile
metadata:
  name:  aws-profile-xyz
  namespace: project-xyz
spec:
  parent:
    kind: CloudProfile
    name: aws-central-cloud-profile
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
status:
  cloudProfile:
    apiVersion: core.gardener.cloud/v1beta1
    kind: CloudProfile
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
```

The rendering is done by a new custom controller registered to the `gardener-controller-manager`. Merge conflicts can not arise during the merge process as they are caught by static validation and an admission plugin for validating the `NamespacedCloudProfile` object.

### Custom RBAC verb

To prevent users from entering arbitrary values in the `kubernetes` and `machineImages` fields, two custom RBAC verbs may be introduced. It can then be checked if the user that is creating or updating the `kubernetes` or `machineImages` field is authorized to do so.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: namespaced-cloud-profile-modify-special-fields
rules:
  - apiGroups: ["core.gardener.cloud"]
    resources: ["namespacedcloudprofiles"]
    verbs: ["modify-spec-kubernetes", "modify-spec-machineImages"]
```

### Adjusting `Shoot`s `cloudProfileName` field

Since `CloudProfiles` and `NamespacedCloudProfiles` are separate API objects, a `CloudProfile` could have the same name as a `NamespacedCloudProfile`. When the Shoot then references the `CloudProfile` two possible profiles could match on the reference. To solve this issue, the `Shoot` object should be extended with a `cloudProfile` field that specifies both the `name` and the `kind` of the referenced `CloudProfile`. However, the existing `cloudProfileName` field should remain intact and default to a `CloudProfile` for a smooth migration. Sane default will also be defined to allow for full backward compatibility with the `cloudProfileName` field.

An example of the new field can be found here:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: project-xyz
  ...
spec:
  ...
  cloudProfile:
    kind: NamespacedCloudProfile
    name: aws-profile-xyz
  ...
```

### Migration path

`CloudProfile`s are not just a central Gardener resource but are also used by Gardener's provider extensions. Therefore, enabling `NamespacedCloudProfiles` is a multi-step process.

1. Rollout `NamespacedCloudProfile` API and validations, rollout Shoot API changes, add a feature gate for both
2. Adjust all provider extensions to understand both `CloudProfile`s as well as `NamespacedCloudProfiles`
3. Migrate `Shoot`s `cloudProfileName` to the `cloudProfile` field but keep `cloudProfileName` in the API for now
4. Enable `NamespacedCloudProfiles` in Gardener

## Outlook

There are a couple of key features to this GEP that fit well into Gardeners development but are not included immediately to keep the scope in line with the concrete defined use cases. These features might be implemented within future GEPs or PRs and would certainly add value to Gardener.

### Cross-project sharing

A use case could be defined where a `NamespacedCloudProfile` might want to be shared across multiple projects and not just be used within the project it was created in. Especially when taking a broader view of Gardeners development with Private Seeds and Cloud in Country, this feature is probably going to be necessary at some point.

This GEP already modifies the `cloudProfile` field in the `Shoot`s spec. To implement cross-project sharing, a `namespace` field could be added to the `cloudProfile` field. It specifies in which project/namespace the selected `CloudProfile` is. Checking if a user is allowed to select the specified `CloudProfile` can be handled in multiple ways and should be specified more concretely when this feature is to be implemented.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: project-xyz
  ...
spec:
  ...
  cloudProfile:
    kind: NamespacedCloudProfile
    name: aws-profile-xyz
    namespace: other-project-abc
  ...
```

### Multi-Level inheritance

A `NamespacedCloudProfile` could inherit from a `NamespacedCloudProfile` that already inherits from a `CloudProfile`. This would enable reusing `NamespacedCloudProfile`s and would aid in deduplication within fields of `NamespacedCloudProfiles`. This feature can be implemented easily but is also excluded for now to stick to the defined use cases.

When enabling multi-level inheritance, the `NamespacedCloudProfile`s `parent` field should also be adjusted to allow for a `NamespacedCloudProfile` as a parent.

```yaml
parent:
  kind: <NamespacedCloudProfile | CloudProfile>
  name: <objects name>
```

In combination with [cross-project sharing](#cross-project-sharing), the `parent` field should also allow for a namespace to be defined with the same reasoning as in [cross-project sharing](#cross-project-sharing).

```yaml
parent:
  kind: <NamespacedCloudProfile | CloudProfile>
  name: <objects name>
  namespace: <(optional) objects namespace if outside project namespace>
```

### `NamespacedCloudProfile`s replacing regular `CloudProfile`s

In the future, once `NamespacedCloudProfiles` have established themselves and found good use in the Gardener landscape and amongst its users, they could replace regular `CloudProfile`s entirely. For this, they would have to include all fields of a `CloudProfile`. Replacing `CloudProfile`s with `NamespacedCloudProfile`s has several benefits. Firstly, Gardener maintainers don't have to maintain, test and operate two almost identical objects. Secondly, it would allow for the central "Gardener provided" `CloudProfile`s to use inheritance. This could enable deduplication in our central `CloudProfiles` as common values do not have to be copied from one `CloudProfile` to another `CloudProfile`. Additionally, it could enable both cloud, as well as on-prem infrastructures to only be visible to defined projects and not to every landscape user.

## Alternatives

### Arbitrary Value Fields

Instead of specifying a `NamespacedCloudProfile` resource, an end user could be allowed to enter arbitrary names in fields such as `machineTypes`. The entry would then, if the user enters a wrong value, throw an error when provisioning the resource at the cloud provider. However, this approach does not seem feasible as the metadata that is specified in a `CloudProfile` like the number of CPUs and amount of RAM is used in the trial clusters (see [Shoot Quotas](https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md#shoot-quotas)) to validate quotas and to enable the scale-from-zero feature. Therefore, an exception would need to be developed for this specific, and possibly other, use cases.

### Namespaced Cloud Profiles by Selection

Instead of using a single inheritance-based approach with the `parent` field, a similar approach as [aggregated cluster roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles) in Kubernetes could be used. In this approach, `CloudProfile`s would be defined without a parent and would be aggregated together. However, this approach is not well suited as it can not clearly define which `CloudProfile` overwrites which fields. A restriction could be defined that when combining multiple `CloudProfile`s, no merge conflicts must be introduced, making the approach more reasonable.
