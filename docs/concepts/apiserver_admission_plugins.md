# Admission Plugins

Similar to the kube-apiserver, the gardener-apiserver comes with a few in-tree managed admission plugins.
If you want to get an overview of the what and why of admission plugins then [this document](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) might be a good start.

This document lists all existing admission plugins with a short explanation of what it is responsible for.

## `ClusterOpenIDConnectPreset`, `OpenIDConnectPreset`

_(both enabled by default)_

These admission controllers react on `CREATE` operations for `Shoot`s.
If the `Shoot` does not specify any OIDC configuration (`.spec.kubernetes.kubeAPIServer.oidcConfig=nil`) then it tries to find a matching `ClusterOpenIDConnectPreset` or `OpenIDConnectPreset`, respectively.
If there are multiples that match then the one with the highest weight "wins".
In this case, the admission controller will default the OIDC configuration in the `Shoot`.

## `ControllerRegistrationResources`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `ControllerRegistration`s.
It validates that there exists only one `ControllerRegistration` in the system that is primarily responsible for a given kind/type resource combination.
This prevents misconfiguration by the Gardener administrator/operator.

## `CustomVerbAuthorizer`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Project`s.
It validates whether the user is bound to a RBAC role with the `modify-spec-tolerations-whitelist` verb in case the user tries to change the `.spec.tolerations.whitelist` field of the respective `Project` resource.
Usually, regular project members are not bound to this custom verb, allowing the Gardener administrator to manage certain toleration whitelists on `Project` basis.

## `DeletionConfirmation`

_(enabled by default)_

This admission controller reacts on `DELETE` operations for `Project`s and `Shoot`s.
It validates that the respective resource is annotated with a deletion confirmation annotation, namely `confirmation.gardener.cloud/deletion=true`.
Only if this annotation is present it allows the `DELETE` operation to pass.
This prevents users from accidental/undesired deletions.

## `ExtensionValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `BackupEntry`s, `BackupBucket`s, `Seed`s, and `Shoot`s.
For all the various extension types in the specifications of these objects, it validates whether there exists a `ControllerRegistration` in the system that is primarily responsible for the stated extension type(s).
This prevents misconfigurations that would otherwise allow users to create such resources with extension types that don't exist in the cluster, effectively leading to failing reconciliation loops.

## `PlantValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Plant`s.
It sets the `gardener.cloud/created-by` annotation for newly created `Plant` resources.
Also, it prevents creating new `Plant` resources in `Project`s that are already have a deletion timestamp.

## `ResourceReferenceManager`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `CloudProfile`s, `Project`s, `SecretBinding`s, `Seed`s, and `Shoot`s.
Generally, it checks whether referred resources stated in the specifications of these objects exist in the system (e.g., if a referenced `Secret` exists).
However, it also has some special behaviours for certain resources:

* `CloudProfile`s: It rejects removing Kubernetes or machine image versions if there is at least one `Shoot` that refers to them.
* `Project`s: It sets the `.spec.createdBy` field for newly created `Project` resources, and defaults the `.spec.owner` field in case it is empty (to the same value of `.spec.createdBy`).
* `Seed`s: It rejects changing the `.spec.settings.shootDNS.enabled` value if there is at least one `Shoot` that refers to this seed.
* `Shoot`s: It sets the `gardener.cloud/created-by=<username>` annotation for newly created `Shoot` resources.

## `SeedValidator`

_(enabled by default)_

This admission controller reacts on `DELETE` operations for `Seed`s.
It checks whether the seed cluster is referenced by a `BackupBucket`(s) and/or `Shoot`(s). If any of this is true, the deletion request is rejected.

## `ShootDNS`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It tries to assign a default domain to the `Shoot` if it gets scheduled to a seed that enables DNS for shoots (`.spec.settings.shootDNS.enabled=true`).
It also validates that the DNS configuration (`.spec.dns`) is not set if the seed disables DNS for shoots.

## `ShootQuotaValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It validates the resource consumption declared in the specification against applicable `Quota` resources.
Only if the applicable `Quota` resources admit the configured resources in the `Shoot` then it allows the request.
Applicable `Quota`s are referred in the `SecretBinding` that is used by the `Shoot`.

## `ShootStateDeletionValidator`

_(enabled by default)_

This admission controller reacts on `DELETE` operations for `ShootState`s.
It prevents the deletion of the respective `ShootState` resource in case the corresponding `Shoot` resource does still exist in the system.
This prevents losing the shoot's data required to recover it / migrate its control plane to a new seed cluster.

## `ShootTolerationRestriction`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It validates the `.spec.tolerations` used in `Shoot`s against the whitelist of its `Project`, or against the whitelist configured in the admission controller's configuration, respectively.
Additionally, it defaults the `.spec.tolerations` in `Shoot`s with those configured in its `Project`, and those configured in the admission controller's configuration, respectively.

## `ShootValidator`

_(enabled by default)_

This admission controller reacts on `CREATE` and `UPDATE` operations for `Shoot`s.
It validates certain configurations in the specification against the referred `CloudProfile` (e.g., machine images, machine types, used Kubernetes version, ...).
Generally, it performs validations that cannot be handled by the static API validation due to their dynamic nature (e.g., when something needs to be checked against referred resources).
Additionally, it takes over certain defaulting tasks (e.g., default machine image for worker pools).
