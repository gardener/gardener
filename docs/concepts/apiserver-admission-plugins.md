---
title: APIServer Admission Plugins
description: A list of all gardener managed admission plugins together with their responsibilities
---

## Overview

Similar to the kube-apiserver, the gardener-apiserver comes with a few in-tree managed admission plugins.
If you want to get an overview of the what and why of admission plugins then [this document](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) might be a good start.

This document lists all existing admission plugins with a short explanation of what it is responsible for.

## `ClusterOpenIDConnectPreset`, `OpenIDConnectPreset`

_(both enabled by default)_

These admission controllers react on `CREATE` operations for `Shoot`s.
If the `Shoot` does not specify any OIDC configuration (`.spec.kubernetes.kubeAPIServer.oidcConfig=nil`), then it tries to find a matching `ClusterOpenIDConnectPreset` or `OpenIDConnectPreset`, respectively.
If there are multiple matches, then the one with the highest weight "wins".
In this case, the admission controller will default the OIDC configuration in the `Shoot`.

## `ControllerRegistrationResources`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `ControllerRegistration`s.
It validates that there exists only one `ControllerRegistration` in the system that is primarily responsible for a given kind/type resource combination.
This prevents misconfiguration by the Gardener administrator/operator.

## `CustomVerbAuthorizer`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Project`s and `NamespacedCloudProfile`s.

For `Project`s it validates whether the user is bound to an RBAC role with the `modify-spec-tolerations-whitelist` verb in case the user tries to change the `.spec.tolerations.whitelist` field of the respective `Project` resource.
Usually, regular project members are not bound to this custom verb, allowing the Gardener administrator to manage certain toleration whitelists on `Project` basis.

For `NamespacedCloudProfile`s, the modification of specific fields also require the user to be bound to an RBAC role with custom verbs.
Please see [this document](../usage/project/namespaced-cloud-profiles.md#field-modification-restrictions) for more information.

## `DeletionConfirmation`

_(enabled by default)_

This admission controller reacts on `DELETE` operations for `Project`s, `Shoot`s, and `ShootState`s.
It validates that the respective resource is annotated with a deletion confirmation annotation, namely `confirmation.gardener.cloud/deletion=true`.
Only if this annotation is present it allows the `DELETE` operation to pass.
This prevents users from accidental/undesired deletions.
In addition, it applies the "four-eyes principle for deletion" concept if the `Project` is configured accordingly.
Find all information about it [in this document](../usage/project/projects.md#four-eyes-principle-for-resource-deletion).

Furthermore, this admission controller reacts on `CREATE` or `UPDATE` operations for `Shoot`s.
It makes sure that the `deletion.gardener.cloud/confirmed-by` annotation is properly maintained in case the `Shoot` deletion is confirmed with above mentioned annotation.

## `ExposureClass`

_(enabled by default)_

This admission controller reacts on `Create` operations for `Shoot`s.
It mutates `Shoot` resources which have an `ExposureClass` referenced by merging both their `shootSelectors` and/or `tolerations` into the `Shoot` resource.

## `ExtensionValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `BackupEntry`s, `BackupBucket`s, `Seed`s, and `Shoot`s.
For all the various extension types in the specifications of these objects, it validates whether there exists a `ControllerRegistration` in the system that is primarily responsible for the stated extension type(s).
This prevents misconfigurations that would otherwise allow users to create such resources with extension types that don't exist in the cluster, effectively leading to failing reconciliation loops.

## `ExtensionLabels`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `BackupBucket`s, `BackupEntry`s, `CloudProfile`s, `NamespacedCloudProfile`s, `Seed`s, `SecretBinding`s, `CredentialsBinding`s, `WorkloadIdentity`s and `Shoot`s. For all the various extension types in the specifications of these objects, it adds a corresponding label in the resource. This would allow extension admission webhooks to filter out the resources they are responsible for and ignore all others. This label is of the form `<extension-type>.extensions.gardener.cloud/<extension-name> : "true"`. For example, an extension label for provider extension type `aws`, looks like `provider.extensions.gardener.cloud/aws : "true"`.

## `ProjectValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` operations for `Project`s.
It prevents creating `Project`s with a non-empty `.spec.namespace` if the value in `.spec.namespace` does not start with `garden-`.

## `ResourceQuota`

_(enabled by default)_

This admission controller enables [object count ResourceQuotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/#object-count-quota) for Gardener resources, e.g. `Shoots`, `SecretBindings`, `Projects`, etc.
> :warning: In addition to this admission plugin, the [ResourceQuota controller](https://github.com/kubernetes/kubernetes/blob/release-1.2/docs/design/admission_control_resource_quota.md#resource-quota-controller) must be enabled for the Kube-Controller-Manager of your Garden cluster.

## `ResourceReferenceManager`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `CloudProfile`s, `Project`s, `SecretBinding`s, `Seed`s, and `Shoot`s.
Generally, it checks whether referred resources stated in the specifications of these objects exist in the system (e.g., if a referenced `Secret` exists).
However, it also has some special behaviours for certain resources:

* `CloudProfile`s: It rejects removing Kubernetes or machine image versions if there is at least one `Shoot` that refers to them.
* `Project`s: It sets the `.spec.createdBy` field for newly created `Project` resources, and defaults the `.spec.owner` field in case it is empty (to the same value of `.spec.createdBy`).
* `Shoot`s: It sets the `gardener.cloud/created-by=<username>` annotation for newly created `Shoot` resources.

## `SeedValidator`

_(enabled by default)_

This admission controller reacts on `DELETE` operations for `Seed`s.
Rejects the deletion if `Shoot`(s) reference the seed cluster.

## `ShootDNS`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It tries to assign a default domain to the `Shoot`.
It also validates the DNS configuration (`.spec.dns`) for shoots.

## `ShootNodeLocalDNSEnabledByDefault`

_(disabled by default)_

This admission controller reacts on `CREATE` operations for `Shoot`s.
If enabled, it will enable node local dns within the shoot cluster (for more information, see [NodeLocalDNS Configuration](../usage/networking/node-local-dns.md)) by setting `spec.systemComponents.nodeLocalDNS.enabled=true` for newly created Shoots.
Already existing Shoots and new Shoots that explicitly disable node local dns (`spec.systemComponents.nodeLocalDNS.enabled=false`)
will not be affected by this admission plugin.

## `ShootQuotaValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It validates the resource consumption declared in the specification against applicable `Quota` resources.
Only if the applicable `Quota` resources admit the configured resources in the `Shoot` then it allows the request.
Applicable `Quota`s are referred in the `SecretBinding` that is used by the `Shoot`.

## `ShootResourceReservation`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It injects the `Kubernetes.Kubelet.KubeReserved` setting for kubelet either as global setting for a shoot or on a per worker pool basis.
If the admission configuration (see [this example](../../example/20-admissionconfig.yaml)) for the `ShootResourceReservation` plugin contains `useGKEFormula: false` (the default), then it sets a static default resource reservation for the shoot.

If `useGKEFormula: true` is set, then the plugin injects resource reservations based on the machine type similar to GKE's [formula for resource reservation](https://cloud.google.com/kubernetes-engine/docs/concepts/plan-node-sizes#resource_reservations) into each worker pool.
Already existing resource reservations are not modified; this also means that resource reservations are not automatically updated if the machine type for a worker pool is changed.
If a shoot contains global resource reservations, then no per worker pool resource reservations are injected.

By default, `useGKEFormula: true` applies to all Shoots.
Operators can provide an optional label selector via the `selector` field to limit which Shoots get worker specific resource reservations injected.

## `ShootVPAEnabledByDefault`

_(disabled by default)_

This admission controller reacts on `CREATE` operations for `Shoot`s.
If enabled, it will enable the managed `VerticalPodAutoscaler` components (for more information, see [Vertical Pod Auto-Scaling](../usage/autoscaling/shoot_autoscaling.md#vertical-pod-auto-scaling))
by setting `spec.kubernetes.verticalPodAutoscaler.enabled=true` for newly created Shoots.
Already existing Shoots and new Shoots that explicitly disable VPA (`spec.kubernetes.verticalPodAutoscaler.enabled=false`)
will not be affected by this admission plugin.

## `ShootTolerationRestriction`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It validates the `.spec.tolerations` used in `Shoot`s against the whitelist of its `Project`, or against the whitelist configured in the admission controller's configuration, respectively.
Additionally, it defaults the `.spec.tolerations` in `Shoot`s with those configured in its `Project`, and those configured in the admission controller's configuration, respectively.

## `ShootValidator`

_(enabled by default)_

This admission controller reacts on `CREATE`, `UPDATE` and `DELETE` operations for `Shoot`s.
It validates certain configurations in the specification against the referred `CloudProfile` (e.g., machine images, machine types, used Kubernetes version, ...).
Generally, it performs validations that cannot be handled by the static API validation due to their dynamic nature (e.g., when something needs to be checked against referred resources).
Additionally, it takes over certain defaulting tasks (e.g., default machine image for worker pools, default Kubernetes version).

## `ShootManagedSeed`

_(enabled by default)_

This admission controller reacts on `UPDATE` and `DELETE` operations for `Shoot`s.
It validates certain configuration values in the specification that are specific to `ManagedSeed`s (e.g. the nginx-addon of the Shoot has to be disabled, the Shoot VPA has to be enabled).
It rejects the deletion if the `Shoot` is referred to by a `ManagedSeed`.

## `ManagedSeedValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `ManagedSeeds`s.
It validates certain configuration values in the specification against the referred `Shoot`, for example Seed provider, network ranges, DNS domain, etc.
Similar to `ShootValidator`, it performs validations that cannot be handled by the static API validation due to their dynamic nature.
Additionally, it performs certain defaulting tasks, making sure that configuration values that are not specified are defaulted to the values of the referred `Shoot`, for example Seed provider, network ranges, DNS domain, etc.

## `ManagedSeedShoot`

_(enabled by default)_

This admission controller reacts on `DELETE` operations for `ManagedSeed`s.
It rejects the deletion if there are `Shoot`s that are scheduled onto the `Seed` that is registered by the `ManagedSeed`.

## `ShootDNSRewriting`

_(disabled by default)_

This admission controller reacts on `CREATE` operations for `Shoot`s.
If enabled, it adds a set of common suffixes configured in its admission plugin configuration to the `Shoot` (`spec.systemComponents.coreDNS.rewriting.commonSuffixes`) (for more information, see [DNS Search Path Optimization](../usage/networking/dns-search-path-optimization.md)).
Already existing `Shoot`s will not be affected by this admission plugin.

## `NamespacedCloudProfileValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `NamespacedCloudProfile`s.
It primarily validates if the referenced parent `CloudProfile` exists in the system. In addition, the admission controller ensures that the `NamespacedCloudProfile` only configures new machine types, and does not overwrite those from the parent `CloudProfile`.
