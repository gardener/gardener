---
title: Gardener Controller Manager
description: Understand where the gardener-controller-manager runs and its functionalities
---

## Overview

The `gardener-controller-manager` (often referred to as "GCM") is a component that runs next to the Gardener API server, similar to the Kubernetes Controller Manager.
It runs several controllers that do not require talking to any seed or shoot cluster.
Also, as of today, it exposes an HTTP server that is serving several health check endpoints and metrics.

This document explains the various functionalities of the `gardener-controller-manager` and their purpose.

## Controllers

### [`Bastion` Controller](../../pkg/controllermanager/controller/bastion)

`Bastion` resources have a limited lifetime which can be extended up to a certain amount by performing a heartbeat on them.
The `Bastion` controller is responsible for deleting expired or rotten `Bastion`s.

- "expired" means a `Bastion` has exceeded its `status.expirationTimestamp`.
- "rotten" means a `Bastion` is older than the configured `maxLifetime`.

The `maxLifetime` defaults to 24 hours and is an option in the `BastionControllerConfiguration` which is part of `gardener-controller-manager`s `ControllerManagerControllerConfiguration`, see [the example config file](../../example/20-componentconfig-gardener-controller-manager.yaml) for details.

The controller also deletes `Bastion`s in case the referenced `Shoot`:

- no longer exists
- is marked for deletion (i.e., have a non-`nil` `.metadata.deletionTimestamp`)
- was migrated to another seed (i.e., `Shoot.spec.seedName` is different than `Bastion.spec.seedName`).

The deletion of `Bastion`s triggers the `gardenlet` to perform the necessary cleanups in the Seed cluster, so some time can pass between deletion and the `Bastion` actually disappearing.
Clients like `gardenctl` are advised to not re-use `Bastion`s whose deletion timestamp has been set already.

Refer to [GEP-15](../proposals/15-manage-bastions-and-ssh-key-pair-rotation.md) for more information on the lifecycle of
`Bastion` resources.

### [`CertificateSigningRequest` Controller](../../pkg/controllermanager/controller/certificatesigningrequest)

After the [gardenlet](./gardenlet.md) gets deployed on the Seed cluster, it needs to establish itself as a trusted party to communicate with the Gardener API server. It runs through a bootstrap flow similar to the [kubelet bootstrap](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/) process.

On startup, the gardenlet uses a `kubeconfig` with a [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) which authenticates it as being part of the `system:bootstrappers` group. This kubeconfig is used to create a [`CertificateSigningRequest`](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/) (CSR) against the Gardener API server.

The controller in `gardener-controller-manager` checks whether the `CertificateSigningRequest` has the expected organization, common name and usages which the gardenlet would request.

It only auto-approves the CSR if the client making the request is allowed to "create" the
`certificatesigningrequests/seedclient` subresource. Clients with the `system:bootstrappers` group are bound to the `gardener.cloud:system:seed-bootstrapper` `ClusterRole`, hence, they have such privileges. As the bootstrap kubeconfig for the gardenlet contains a bootstrap token which is authenticated as being part of the [`systems:bootstrappers` group](../../charts/gardener/controlplane/charts/application/templates/clusterrolebinding-seed-bootstrapper.yaml), its created CSR gets auto-approved.

### [`CloudProfile` Controller](../../pkg/controllermanager/controller/cloudprofile)

`CloudProfile`s are essential when it comes to reconciling `Shoot`s since they contain constraints (like valid machine types, Kubernetes versions, or machine images) and sometimes also some global configuration for the respective environment (typically via provider-specific configuration in `.spec.providerConfig`).

Consequently, to ensure that `CloudProfile`s in-use are always present in the system until the last referring `Shoot` or `NamespacedCloudProfile` gets deleted, the controller adds a finalizer which is only released when there is no `Shoot` or `NamespacedCloudProfile` referencing the `CloudProfile` anymore.

### [`NamespacedCloudProfile` Controller](../../pkg/controllermanager/controller/namespacedcloudprofile)

`NamespacedCloudProfile`s provide a project-scoped extension to `CloudProfile`s, allowing for adjustments of a parent `CloudProfile` (e.g. by overriding expiration dates of Kubernetes versions or machine images). This allows for modifications without global project visibility. Like `CloudProfile`s do in their spec, `NamespacedCloudProfile`s also expose the resulting `Shoot` constraints as a `CloudProfileSpec` in their status.

The controller ensures that `NamespacedCloudProfile`s in-use remain present in the system until the last referring `Shoot` is deleted by adding a finalizer that is only released when there is no `Shoot` referencing the `NamespacedCloudProfile` anymore.

### [`ControllerDeployment` Controller](../../pkg/controllermanager/controller/controllerdeployment)

Extensions are registered in the garden cluster via `ControllerRegistration` and deployment of respective extensions are specified via `ControllerDeployment`. For more info refer to [Registering Extension Controllers](../extensions/controllerregistration.md).

This controller ensures that `ControllerDeployment` in-use always exists until the last `ControllerRegistration` referencing them gets deleted. The controller adds a finalizer which is only released when there is no `ControllerRegistration` referencing the `ControllerDeployment` anymore.

### [`ControllerRegistration` Controller](../../pkg/controllermanager/controller/controllerregistration)

The `ControllerRegistration` controller makes sure that the required [Gardener Extensions](../README.md#extensions) specified by the [`ControllerRegistration`](../extensions/controllerregistration.md) resources are present in the seed clusters.
It also takes care of the creation and deletion of `ControllerInstallation` objects for a given seed cluster.
The controller has three reconciliation loops.

#### ["Main" Reconciler](../../pkg/controllermanager/controller/controllerregistration/seed)

This reconciliation loop watches the `Seed` objects and determines which `ControllerRegistration`s are required for them and reconciles the corresponding `ControllerInstallation` resources to reach the determined state.
To begin with, it computes the kind/type combinations of extensions required for the seed.
For this, the controller examines a live list of `ControllerRegistration`s, `ControllerInstallation`s, `BackupBucket`s, `BackupEntry`s, `Shoot`s, and `Secret`s from the garden cluster.
For example, it examines the shoots running on the seed and deducts the kind/type, like `Infrastructure/gcp`.
The seed (`seed.spec.provider.type`) and DNS (`seed.spec.dns.provider.type`) provider types are considered when calculating the list of required `ControllerRegistration`s, as well.
It also decides whether they should always be deployed based on the `.spec.deployment.policy`.
For the configuration options, please see this [section](../extensions/controllerregistration.md#deployment-configuration-options).

Based on these required combinations, each of them are mapped to `ControllerRegistration` objects and then to their corresponding `ControllerInstallation` objects (if existing).
The controller then creates or updates the required `ControllerInstallation` objects for the given seed.
It also deletes every existing `ControllerInstallation` whose referenced `ControllerRegistration` is not part of the required list.
For example, if the shoots in the seed are no longer using the DNS provider `aws-route53`, then the controller proceeds to delete the respective `ControllerInstallation` object.

#### ["`ControllerRegistration` Finalizer" Reconciler](../../pkg/controllermanager/controller/controllerregistration/controllerregistrationfinalizer)

This reconciliation loop watches the `ControllerRegistration` resource and adds finalizers to it when they are created.
In case a deletion request comes in for the resource, i.e., if a `.metadata.deletionTimestamp` is set, it actively scans for a `ControllerInstallation` resource using this `ControllerRegistration`, and decides whether the deletion can be allowed.
In case no related `ControllerInstallation` is present, it removes the finalizer and marks it for deletion.

#### ["`Seed` Finalizer" Reconciler](../../pkg/controllermanager/controller/controllerregistration/seedfinalizer)

This loop also watches the `Seed` object and adds finalizers to it at creation.
If a `.metadata.deletionTimestamp` is set for the seed, then the controller checks for existing `ControllerInstallation` objects which reference this seed.
If no such objects exist, then it removes the finalizer and allows the deletion.

#### ["Extension `ClusterRole`" Reconciler](../../pkg/controllermanager/controller/controllerregistration/extensionclusterrole)

This reconciler watches two resources in the garden cluster:

- `ClusterRole`s labelled with `authorization.gardener.cloud/custom-extensions-permissions=true`
- `ServiceAccount`s in seed namespaces matching the selector provided via the `authorization.gardener.cloud/extensions-serviceaccount-selector` annotation of such `ClusterRole`s.

Its core task is to maintain a `ClusterRoleBinding` resource referencing the respective `ClusterRole`.
This gets bound to all `ServiceAccount`s in seed namespaces whose labels match the selector provided via the `authorization.gardener.cloud/extensions-serviceaccount-selector` annotation of such `ClusterRole`s.

You can read more about the purpose of this reconciler in [this document](../extensions/garden-api-access.md#additional-permissions).

### [`CredentialsBinding` Controller](../../pkg/controllermanager/controller/credentialsbinding)

`CredentialsBinding`s reference `Secret`s, `WorkloadIdentity`s and `Quota`s and are themselves referenced by `Shoot`s.

The controller adds finalizers to the referenced objects to ensure they don't get deleted while still being referenced.
Similarly, to ensure that `CredentialsBinding`s in-use are always present in the system until the last referring `Shoot` gets deleted, the controller adds a finalizer which is only released when there is no `Shoot` referencing the `CredentialsBinding` anymore.

Referenced `Secret`s and `WorkloadIdentity`s will also be labeled with `provider.shoot.gardener.cloud/<type>=true`, where `<type>` is the value of the `.provider.type` of the `CredentialsBinding`.
Also, all referenced `Secret`s and `WorkloadIdentity`s, as well as `Quota`s, will be labeled with `reference.gardener.cloud/credentialsbinding=true` to allow for easily filtering for objects referenced by `CredentialsBinding`s.

### [`Event` Controller](../../pkg/controllermanager/controller/event)

With the Gardener Event Controller, you can prolong the lifespan of events related to Shoot clusters.
This is an optional controller which will become active once you provide the below mentioned configuration.

All events in K8s are deleted after a configurable time-to-live (controlled via a [kube-apiserver argument](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/) called `--event-ttl` (defaulting to 1 hour)).
The need to prolong the time-to-live for Shoot cluster events frequently arises when debugging customer issues on live systems.
This controller leaves events involving Shoots untouched, while deleting all other events after a configured time.
In order to activate it, provide the following configuration:

* `concurrentSyncs`: The amount of goroutines scheduled for reconciling events.
* `ttlNonShootEvents`: When an event reaches this time-to-live it gets deleted unless it is a Shoot-related event (defaults to `1h`, equivalent to the `event-ttl` default).

> :warning: In addition, you should also configure the `--event-ttl` for the kube-apiserver to define an upper-limit of how long Shoot-related events should be stored. The `--event-ttl` should be larger than the `ttlNonShootEvents` or this controller will have no effect.

### [`ExposureClass` Controller](../../pkg/controllermanager/controller/exposureclass)

`ExposureClass` abstracts the ability to expose a Shoot clusters control plane in certain network environments (e.g. corporate networks, DMZ, internet) on all Seeds or a subset of the Seeds. For more information, see [ExposureClasses](../usage/networking/exposureclasses.md).

Consequently, to ensure that `ExposureClass`es in-use are always present in the system until the last referring `Shoot` gets deleted, the controller adds a finalizer which is only released when there is no `Shoot` referencing the `ExposureClass` anymore.

### [`ManagedSeedSet` Controller](../../pkg/controllermanager/controller/managedseedset)

`ManagedSeedSet` objects maintain a stable set of replicas of `ManagedSeed`s, i.e. they guarantee the availability of a specified number of identical `ManagedSeed`s on an equal number of identical `Shoot`s.
The `ManagedSeedSet` controller creates and deletes `ManagedSeed`s and `Shoot`s in response to changes to the replicas and selector fields. For more information, refer to the [`ManagedSeedSet` proposal document](../proposals/13-automated-seed-management.md#managedseedsets).

1. The reconciler first gets all the replicas of the given `ManagedSeedSet` in the `ManagedSeedSet`'s namespace and with the matching selector. Each replica is a struct that contains a `ManagedSeed`, its corresponding `Seed` and `Shoot` objects.
1. Then the pending replica is retrieved, if it exists.
1. Next it determines the ready, postponed, and deletable replicas.
    - A replica is considered `ready` when a `Seed` owned by a `ManagedSeed` has been registered either directly or by deploying `gardenlet` into a `Shoot`, the `Seed` is `Ready` and the `Shoot`'s status is `Healthy`.
    - If a replica is not ready and it is not pending, i.e. it is not specified in the `ManagedSeed`'s `status.pendingReplica` field, then it is added to the `postponed` replicas.
    - A replica is deletable if it has no scheduled `Shoot`s and the replica's `Shoot` and `ManagedSeed` do not have the `seedmanagement.gardener.cloud/protect-from-deletion` annotation.
1. Finally, it checks the actual and target replica counts. If the actual count is less than the target count, the controller scales up the replicas by creating new replicas to match the desired target count. If the actual count is more than the target, the controller deletes replicas to match the desired count. Before scale-out or scale-in, the controller first reconciles the pending replica (there can always only be one) and makes sure the replica is ready before moving on to the next one.
    * `Scale-out`(actual count < target count)
        - During the scale-out phase, the controller first creates the `Shoot` object from the `ManagedSeedSet`'s `spec.shootTemplate` field and adds the replica to the `status.pendingReplica` of the `ManagedSeedSet`.
        - For the subsequent reconciliation steps, the controller makes sure that the pending replica is ready before proceeding to the next replica. Once the `Shoot` is created successfully, the `ManagedSeed` object is created from the `ManagedSeedSet`'s `spec.template`. The `ManagedSeed` object is reconciled by the `ManagedSeed` controller and a `Seed` object is created for the replica. Once the replica's `Seed` becomes ready and the `Shoot` becomes healthy, the replica also becomes ready.
    * `Scale-in`(actual count > target count)
        - During the scale-in phase, the controller first determines the replica that can be deleted. From the deletable replicas, it chooses the one with the lowest priority and deletes it. Priority is determined in the following order:
            - First, compare replica statuses. Replicas with "less advanced" status are considered lower priority. For example, a replica with `StatusShootReconciling` status has a lower value than a replica with `StatusShootReconciled` status. Hence, in this case, a replica with a `StatusShootReconciling` status will have lower priority and will be considered for deletion.
            - Then, the replicas are compared with the readiness of their `Seed`s. Replicas with non-ready `Seed`s are considered lower priority.
            - Then, the replicas are compared with the health statuses of their `Shoot`s. Replicas with "worse" statuses are considered lower priority.
            - Finally, the replica ordinals are compared. Replicas with lower ordinals are considered lower priority.

### [`Quota` Controller](../../pkg/controllermanager/controller/quota)

`Quota` object limits the resources consumed by shoot clusters either per provider secret or per project/namespace.

Consequently, to ensure that `Quota`s in-use are always present in the system until the last `SecretBinding` or `CredentialsBinding` that references them gets deleted, the controller adds a finalizer which is only released when there is no `SecretBinding` or `CredentialsBinding` referencing the `Quota` anymore.

### [`Project` Controller](../../pkg/controllermanager/controller/project)

There are multiple controllers responsible for different aspects of `Project` objects.
Please also refer to the [`Project` documentation](../usage/project/projects.md).

#### ["Main" Reconciler](../../pkg/controllermanager/controller/project/project)

This reconciler manages a dedicated `Namespace` for each `Project`.
The namespace name can either be specified explicitly in `.spec.namespace` (must be prefixed with `garden-`) or it will be determined by the controller.
If `.spec.namespace` is set, it tries to create it. If it already exists, it tries to adopt it.
This will only succeed if the `Namespace` was previously labeled with `gardener.cloud/role=project` and `project.gardener.cloud/name=<project-name>`.
This is to prevent end-users from being able to adopt arbitrary namespaces and escalate their privileges, e.g. the `kube-system` namespace.

After the namespace was created/adopted, the controller creates several `ClusterRole`s and `ClusterRoleBinding`s that allow the project members to access related resources based on their roles.
These RBAC resources are prefixed with `gardener.cloud:system:project{-member,-viewer}:<project-name>`.
Gardener administrators and extension developers can define their own roles. For more information, see [Extending Project Roles](../extensions/project-roles.md) for more information.

In addition, operators can configure the Project controller to maintain a default [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) for project namespaces.
Quotas can especially limit the creation of user facing resources, e.g. `Shoots`, `SecretBindings`, `CredentialsBinding`, `Secrets` and thus protect the garden cluster from massive resource exhaustion but also enable operators to align quotas with respective enterprise policies.

> :warning: **Gardener itself is not exempted from configured quotas**. For example, Gardener creates `Secrets` for every shoot cluster in the project namespace and at the same time increases the available quota count. Please mind this additional resource consumption.

The controller configuration provides a template section `controllers.project.quotas` where such a ResourceQuota (see the example below) can be deposited.

```yaml
controllers:
  project:
    quotas:
    - config:
        apiVersion: v1
        kind: ResourceQuota
        spec:
          hard:
            count/shoots.core.gardener.cloud: "100"
            count/secretbindings.core.gardener.cloud: "10"
            count/credentialsbindings.security.gardener.cloud: "10"
            count/secrets: "800"
      projectSelector: {}
```

The Project controller takes the specified `config` and creates a `ResourceQuota` with the name `gardener` in the project namespace.
If a `ResourceQuota` resource with the name `gardener` already exists, the controller will only update fields in `spec.hard` which are **unavailable** at that time.
This is done to configure a default `Quota` in all projects but to allow manual quota increases as the projects' demands increase.
`spec.hard` fields in the `ResourceQuota` object that are not present in the configuration are removed from the object.
Labels and annotations on the `ResourceQuota` `config` get merged with the respective fields on existing `ResourceQuota`s.
An optional `projectSelector` narrows down the amount of projects that are equipped with the given `config`.
If multiple configs match for a project, then only the first match in the list is applied to the project namespace.

The `.status.phase` of the `Project` resources is set to `Ready` or `Failed` by the reconciler to indicate whether the reconciliation loop was performed successfully.
Also, it generates `Event`s to provide further information about its operations.

When a `Project` is marked for deletion, the controller ensures that there are no `Shoots` left in the project namespace.
Once all `Shoots` are gone, the `Namespace` and `Project` are released.

#### ["Stale Projects" Reconciler](../../pkg/controllermanager/controller/project/stale)

As Gardener is a large-scale Kubernetes as a Service, it is designed for being used by a large amount of end-users.
Over time, it is likely to happen that some of the hundreds or thousands of `Project` resources are no longer actively used.

Gardener offers the "stale projects" reconciler which will take care of identifying such stale projects, marking them with a "warning", and eventually deleting them after a certain time period.
This reconciler is enabled by default and works as follows:

1. Projects are considered as "stale"/not actively used when all of the following conditions apply: The namespace associated with the `Project` does not have any...
    1. `Shoot` resources.
    1. `BackupEntry` resources.
    1. `Secret` resources that are referenced by a `SecretBinding` or a `CredentialsBinding` that is in use by a `Shoot` (not necessarily in the same namespace).
    1. `Quota` resources that are referenced by a `SecretBinding` or a `CredentialsBinding` that is in use by a `Shoot` (not necessarily in the same namespace).
    1. The time period when the project was used for the last time (`status.lastActivityTimestamp`) is longer than the configured `minimumLifetimeDays`

If a project is considered "stale", then its `.status.staleSinceTimestamp` will be set to the time when it was first detected to be stale.
If it gets actively used again, this timestamp will be removed.
After some time, the `.status.staleAutoDeleteTimestamp` will be set to a timestamp after which Gardener will auto-delete the `Project` resource if it still is not actively used.

The component configuration of the `gardener-controller-manager` offers to configure the following options:

* `minimumLifetimeDays`: Don't consider newly created `Project`s as "stale" too early to give people/end-users some time to onboard and get familiar with the system. The "stale project" reconciler won't set any timestamp for `Project`s younger than `minimumLifetimeDays`. When you change this value, then projects marked as "stale" may be no longer marked as "stale" in case they are young enough, or vice versa.
* `staleGracePeriodDays`: Don't compute auto-delete timestamps for stale `Project`s that are unused for less than `staleGracePeriodDays`. This is to not unnecessarily make people/end-users nervous "just because" they haven't actively used their `Project` for a given amount of time. When you change this value, then already assigned auto-delete timestamps may be removed if the new grace period is not yet exceeded.
* `staleExpirationTimeDays`: Expiration time after which stale `Project`s are finally auto-deleted (after `.status.staleSinceTimestamp`). If this value is changed and an auto-delete timestamp got already assigned to the projects, then the new value will only take effect if it's increased. Hence, decreasing the `staleExpirationTimeDays` will not decrease already assigned auto-delete timestamps.

> Gardener administrators/operators can exclude specific `Project`s from the stale check by annotating the related `Namespace` resource with `project.gardener.cloud/skip-stale-check=true`.

#### ["Activity" Reconciler](../../pkg/controllermanager/controller/project/activity)

Since the other two reconcilers are unable to actively monitor the relevant objects that are used in a `Project` (`Shoot`, `Secret`, etc.), there could be a situation where the user creates and deletes objects in a short period of time. In that case, the `Stale Project Reconciler` could not see that there was any activity on that project and it will still mark it as a `Stale`, even though it is actively used.

The `Project Activity Reconciler` is implemented to take care of such cases. An event handler will notify the reconciler for any activity and then it will update the `status.lastActivityTimestamp`. This update will also trigger the `Stale Project Reconciler`.

### [`SecretBinding` Controller](../../pkg/controllermanager/controller/secretbinding)

`SecretBinding`s reference `Secret`s and `Quota`s and are themselves referenced by `Shoot`s.
The controller adds finalizers to the referenced objects to ensure they don't get deleted while still being referenced.
Similarly, to ensure that `SecretBinding`s in-use are always present in the system until the last referring `Shoot` gets deleted, the controller adds a finalizer which is only released when there is no `Shoot` referencing the `SecretBinding` anymore.

Referenced `Secret`s will also be labeled with `provider.shoot.gardener.cloud/<type>=true`, where `<type>` is the value of the `.provider.type` of the `SecretBinding`.
Also, all referenced `Secret`s, as well as `Quota`s, will be labeled with `reference.gardener.cloud/secretbinding=true` to allow for easily filtering for objects referenced by `SecretBinding`s.

### [`Seed` Controller](../../pkg/controllermanager/controller/seed)

The Seed controller in the `gardener-controller-manager` reconciles `Seed` objects with the help of the following reconcilers.

#### ["Main" Reconciler](../../pkg/controllermanager/controller/seed/secrets)

This reconciliation loop takes care of seed related operations in the garden cluster. When a new `Seed` object is created,
the reconciler creates a new `Namespace` in the garden cluster `seed-<seed-name>`. `Namespaces` dedicated to single
seed clusters allow us to segregate access permissions i.e., a `gardenlet` must not have permissions to access objects in
all `Namespaces` in the garden cluster.
There are objects in a Garden environment which are created once by the operator e.g., default domain secret,
alerting credentials, and are required for operations happening in the `gardenlet`. Therefore, we not only need a seed specific
`Namespace` but also a copy of these "shared" objects.

The "main" reconciler takes care about this replication:

| Kind   | Namespace | Label Selector      |
|--------|-----------|---------------------|
| Secret | garden    | gardener.cloud/role |

#### ["Backup Buckets Check" Reconciler](../../pkg/controllermanager/controller/seed/backupbucketscheck)

Every time a `BackupBucket` object is created or updated, the referenced `Seed` object is enqueued for reconciliation.
It's the reconciler's task to check the `status` subresource of all existing `BackupBucket`s that reference this `Seed`.
If at least one `BackupBucket` has `.status.lastError != nil`, the `BackupBucketsReady` condition on the `Seed` will be set to `False`, and consequently the `Seed` is considered as `NotReady`.
If the `SeedBackupBucketsCheckControllerConfiguration` (which is part of `gardener-controller-manager`s component configuration) contains a `conditionThreshold` for the `BackupBucketsReady`, the condition will instead first be set to `Progressing` and eventually to `False` once the `conditionThreshold` expires. See [the example config file](../../example/20-componentconfig-gardener-controller-manager.yaml) for details.
Once the `BackupBucket` is healthy again, the seed will be re-queued and the condition will turn `true`.

#### ["Extensions Check" Reconciler](../../pkg/controllermanager/controller/seed/extensionscheck)

This reconciler reconciles `Seed` objects and checks whether all `ControllerInstallation`s referencing them are in a healthy state.
Concretely, all three conditions `Valid`, `Installed`, and `Healthy` must have status `True` and the `Progressing` condition must have status `False`.
Based on this check, it maintains the `ExtensionsReady` condition in the respective `Seed`'s `.status.conditions` list.

#### ["Lifecycle" Reconciler](../../pkg/controllermanager/controller/seed/lifecycle)

The "Lifecycle" reconciler processes `Seed` objects which are enqueued every 10 seconds in order to check if the responsible
`gardenlet` is still responding and operable. Therefore, it checks renewals via `Lease` objects of the seed in the garden cluster
which are renewed regularly by the `gardenlet`.

In case a `Lease` is not renewed for the configured amount in `config.controllers.seed.monitorPeriod.duration`:

1. The reconciler assumes that the `gardenlet` stopped operating and updates the `GardenletReady` condition to `Unknown`.
2. Additionally, the conditions and constraints of all `Shoot` resources scheduled on the affected seed are set to `Unknown` as well,
   because a striking `gardenlet` won't be able to maintain these conditions any more.
3. If the gardenlet's client certificate has expired (identified based on the `.status.clientCertificateExpirationTimestamp` field in the `Seed` resource) and if it is managed by a `ManagedSeed`, then this will be triggered for a reconciliation. This will trigger the bootstrapping process again and allows gardenlets to obtain a fresh client certificate.

### [`Shoot` Controller](../../pkg/controllermanager/controller/shoot)

#### ["Conditions" Reconciler](../../pkg/controllermanager/controller/shoot/conditions)

In case the reconciled `Shoot` is registered via a `ManagedSeed` as a seed cluster, this reconciler merges the conditions in the respective `Seed`'s `.status.conditions` into the `.status.conditions` of the `Shoot`.
This is to provide a holistic view on the status of the registered seed cluster by just looking at the `Shoot` resource.

#### ["Hibernation" Reconciler](../../pkg/controllermanager/controller/shoot/hibernation)

This reconciler is responsible for hibernating or awakening shoot clusters based on the schedules defined in their `.spec.hibernation.schedules`.
It ignores [failed `Shoot`s](../usage/shoot/shoot_status.md#last-operation) and those marked for deletion.

#### ["Maintenance" Reconciler](../../pkg/controllermanager/controller/shoot/maintenance)

This reconciler is responsible for maintaining shoot clusters based on the time window defined in their `.spec.maintenance.timeWindow`.
It might auto-update the Kubernetes version or the operating system versions specified in the worker pools (`.spec.provider.workers`).
It could also add some operation or task annotations. For more information, see [Shoot Maintenance](../usage/shoot/shoot_maintenance.md).

#### ["Quota" Reconciler](../../pkg/controllermanager/controller/shoot/quota)

This reconciler might auto-delete shoot clusters in case their referenced `SecretBinding` or `CredentialsBinding` is itself referencing a `Quota` with `.spec.clusterLifetimeDays != nil`.
If the shoot cluster is older than the configured lifetime, then it gets deleted.
It maintains the expiration time of the `Shoot` in the value of the `shoot.gardener.cloud/expiration-timestamp` annotation.
This annotation might be overridden, however only by at most twice the value of the `.spec.clusterLifetimeDays`.

#### ["Reference" Reconciler](../../pkg/controllermanager/controller/shoot/reference)

Shoot objects may specify references to other objects in the garden cluster which are required for certain features.
For example, users can configure various DNS providers via `.spec.dns.providers` and usually need to refer to a corresponding `Secret` with valid DNS provider credentials inside.
Such objects need a special protection against deletion requests as long as they are still being referenced by one or multiple shoots.

Therefore, this reconciler checks `Shoot`s for referenced objects and adds the finalizer `gardener.cloud/reference-protection` to their `.metadata.finalizers` list.
The reconciled `Shoot` also gets this finalizer to enable a proper garbage collection in case the `gardener-controller-manager` is offline at the moment of an incoming deletion request.
When an object is not actively referenced anymore because the `Shoot` specification has changed or all related shoots were deleted (are in deletion), the controller will remove the added finalizer again so that the object can safely be deleted or garbage collected.

This reconciler inspects the following references:

- Admission plugin kubeconfig `Secret`s (`.spec.kubernetes.kubeAPIServer.admissionPlugins[].kubeconfigSecretName`)
- Audit policy `ConfigMap`s (`.spec.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef`)
- DNS provider `Secret`s (`.spec.dns.providers[].secretName`)
- Structured authentication `ConfigMap`s (`.spec.kubernetes.kubeAPIServer.structuredAuthentication.configMapName`)
- Structured authorization `ConfigMap`s (`.spec.kubernetes.kubeAPIServer.structuredAuthorization.configMapName`)
- Structured authorization kubeconfig `Secret`s (`.spec.kubernetes.kubeAPIServer.structuredAuthorization.kubeconfigs[].secretName`)
- `Secret`s and `ConfigMap`s from `.spec.resources[]`

Further checks might be added in the future.

#### ["Retry" Reconciler](../../pkg/controllermanager/controller/shoot/retry)

This reconciler is responsible for retrying certain failed `Shoot`s.
Currently, the reconciler retries only failed `Shoot`s with an error code `ERR_INFRA_RATE_LIMITS_EXCEEDED`. See [Shoot Status](../usage/shoot/shoot_status.md#error-codes) for more details.

#### ["Status Label" Reconciler](../../pkg/controllermanager/controller/shoot/statuslabel)

This reconciler is responsible for maintaining the `shoot.gardener.cloud/status` label on `Shoot`s. See [Shoot Status](../usage/shoot/shoot_status.md#status-label) for more details.

#### ["Migration" Reconciler](../../pkg/controllermanager/controller/shoot/migration)

This reconciler is triggered for `Shoot`s currently in migration (i.e., `.spec.seedName != .status.seedName`).
It maintains the `ReadyForMigration` constraint in the `.status.constraints[]` list.
A `Shoot` is considered ready for migration if the destination `Seed` is up-to-date and healthy.

The main purpose of this constraint is to allow the `gardenlet` running in the source seed cluster to check if it can start with the migration flow without that it needs to directly read the destination `Seed` resource (for which it won't have permissions).

#### ["ShootState Finalizer" Reconciler](../../pkg/controllermanager/controller/shoot/state/finalizer)

This reconciler is responsible for managing a `ShootState` finalizer that
ensures the object existence during migration of `Shoot`s control plane
to another `Seed`.

The main goal is to keep the `ShootState` present during the `Migrate` and
`Restore` operations that are not yet finished successfully.
