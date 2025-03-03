---
title: NamespacedCloudProfiles
---

# `NamespacedCloudProfile`s

`NamespacedCloudProfile`s are resources in Gardener that allow project-level customization of `CloudProfile`s.
They enable project administrators to create and manage cloud profiles specific to their projects and reduce the operational burden on central Gardener operators.
As opposed to `CloudProfile`s, `NamespacedCloudProfile`s are namespaced and thus limit configuration options for `Shoot`s, such as special machine types, to the associated project only.
These profiles inherit from a parent `CloudProfile` and can override or extend certain fields while maintaining backward compatibility.

Project viewers have the permission to see `NamespacedCloudProfile`s associated with a particular project.
Project administrators can generally create, edit, or delete `NamespacedCloudProfile`s but with some exceptions (see the [restrictions](#field-modification-restrictions) outlined below).

When creating or updating a `Shoot`, the cloud profile reference can be set to point to a `NamespacedCloudProfile`, allowing for more granular and project-specific configurations.
The modification of a `Shoot`'s cloud profile reference is restricted to switching within the same profile hierarchy, i.e. from a `CloudProfile` to a descendant `NamespacedCloudProfile`, from a `NamespacedCloudProfile` to its parent `CloudProfile` and between `NamespacedCloudProfile`s having the same `CloudProfile` parent.
Changing the reference from one `CloudProfile` or descendant `NamespacedCloudProfile` to another `CloudProfile` or descendant `NamespacedCloudProfile` is not allowed.

The usage of `NamespacedCloudProfile`s is currently subject to a beta feature gate and is enabled by default.
It requires the enabled provider extensions to support the feature as well.
The feature gate can be disabled by passing the `--feature-gates=NamespacedCloudProfiles=false` flag to the Gardener API server.

Please see [this](../../../example/35-namespacedcloudprofile.yaml) example manifest and [GEP-25](../../proposals/25-namespaced-cloud-profiles.md) for additional information.

## Field Modification Restrictions

In order to make changes to specific fields in the `NamespacedCloudProfile`, a user must be granted custom RBAC verbs.
Modifications of these fields need to be performed with caution and might require additional validation steps or accompanying changes.
By default, only landscape operators have the permission to change these fields, as they are usually able to judge the implications.

Changing the following fields require the corresponding custom verbs:
* For changing the `.spec.kubernetes` field, the custom verb `modify-spec-kubernetes` is required.
* For changing the `.spec.machineImages` field, the custom verb `modify-spec-machineimages` is required.
* For changing the `.spec.providerConfig` field, the custom verb `modify-spec-providerconfig` is required.

The assignment of these custom verbs can be achieved by creating a `ClusterRole` and a `RoleBinding` like in the following example:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: namespacedcloudprofile-kubernetes
rules:
- apiGroups: ["core.gardener.cloud"] 
  resources: ["namespacedcloudprofiles"]
  verbs: ["modify-spec-kubernetes"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: edit-kubernetes
  namespace: dev
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: namespacedcloudprofile-kubernetes
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: alice.doe@example.com
```
