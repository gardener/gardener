---
title: gardenlet
description: Understand how the gardenlet, the primary "agent" on every seed cluster, works and learn more about the different Gardener components
---

## Overview

Gardener is implemented using the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/):
It uses custom controllers that act on our own custom resources,
and apply Kubernetes principles to manage clusters instead of containers.
Following this analogy, you can recognize components of the Gardener architecture
as well-known Kubernetes components, for example, shoot clusters can be compared with pods,
and seed clusters can be seen as worker nodes.

The following Gardener components play a similar role as the corresponding components
in the Kubernetes architecture:

| Gardener Component            | Kubernetes Component      |
|-------------------------------|---------------------------|
| `gardener-apiserver`          | `kube-apiserver`          |
| `gardener-controller-manager` | `kube-controller-manager` |
| `gardener-scheduler`          | `kube-scheduler`          |
| `gardenlet`                   | `kubelet`                 |

Similar to how the `kube-scheduler` of Kubernetes finds an appropriate node
for newly created pods, the `gardener-scheduler` of Gardener finds an appropriate seed cluster
to host the control plane for newly ordered clusters.
By providing multiple seed clusters for a region or provider, and distributing the workload,
Gardener also reduces the blast radius of potential issues.

Kubernetes runs a primary "agent" on every node, the kubelet,
which is responsible for managing pods and containers on its particular node.
Decentralizing the responsibility to the kubelet has the advantage that the overall system
is scalable. Gardener achieves the same for cluster management by using a **gardenlet**
as a primary "agent" on every seed cluster, and is only responsible for shoot clusters
located in its particular seed cluster:

![Counterparts in the Gardener Architecture and the Kubernetes Architecture](images/gardenlet-architecture-similarities.png)

The `gardener-controller-manager` has controllers to manage resources of the Gardener API. However, instead of letting the `gardener-controller-manager` talk directly to seed clusters or shoot clusters, the responsibility isn’t only delegated to the gardenlet, but also managed using a reversed control flow: It's up to the gardenlet to contact the Gardener API server, for example, to share a status for its managed seed clusters.

Reversing the control flow allows placing seed clusters or shoot clusters behind firewalls without the necessity of direct access via VPN tunnels anymore.

![Reversed Control Flow Using a gardenlet](images/gardenlet-architecture-detailed.png)

## TLS Bootstrapping

Kubernetes doesn’t manage worker nodes itself, and it’s also not
responsible for the lifecycle of the kubelet running on the workers.
Similarly, Gardener doesn’t manage seed clusters itself,
so it is also not responsible for the lifecycle of the gardenlet running on the seeds.
As a consequence, both the gardenlet and the kubelet need to prepare
a trusted connection to the Gardener API server
and the Kubernetes API server correspondingly.

To prepare a trusted connection between the gardenlet
and the Gardener API server, the gardenlet initializes
a bootstrapping process after you deployed it into your seed clusters:

1. The gardenlet starts up with a bootstrap `kubeconfig`
   having a bootstrap token that allows to create `CertificateSigningRequest` (CSR) resources.

2. After the CSR is signed, the gardenlet downloads
   the created client certificate, creates a new `kubeconfig` with it,
   and stores it inside a `Secret` in the seed cluster.

3. The gardenlet deletes the bootstrap `kubeconfig` secret,
    and starts up with its new `kubeconfig`.

4. The gardenlet starts normal operation.

The `gardener-controller-manager` runs a control loop
that automatically signs CSRs created by gardenlets.

> The gardenlet bootstrapping process is based on the
> kubelet bootstrapping process. More information:
> [Kubelet's TLS bootstrapping](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/).

If you don't want to run this bootstrap process, you can create
a `kubeconfig` pointing to the garden cluster for the gardenlet yourself,
and use the field `gardenClientConnection.kubeconfig` in the
gardenlet configuration to share it with the gardenlet.

## gardenlet Certificate Rotation

The certificate used to authenticate the gardenlet against the API server
has a certain validity based on the configuration of the garden cluster
(`--cluster-signing-duration` flag of the `kube-controller-manager` (default `1y`)).

> You can also configure the validity for the client certificate by specifying `.gardenClientConnection.kubeconfigValidity.validity` in the gardenlet's component configuration.
> Note that changing this value will only take effect when the kubeconfig is rotated again (it is not picked up immediately).
> The minimum validity is `10m` (that's what is enforced by the `CertificateSigningRequest` API in Kubernetes which is used by the gardenlet).

By default, after about 70-90% of the validity has expired, the gardenlet tries to automatically replace
the current certificate with a new one (certificate rotation).

> You can change these boundaries by specifying `.gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentage{Min,Max}` in the gardenlet's component configuration.

To use a certificate rotation, you need to specify the secret to store
the `kubeconfig` with the rotated certificate in the field
`.gardenClientConnection.kubeconfigSecret` of the
gardenlet [component configuration](#component-configuration).

### Rotate Certificates Using Bootstrap `kubeconfig`

If the gardenlet created the certificate during the initial TLS Bootstrapping
using the Bootstrap `kubeconfig`, certificates can be rotated automatically.
The same control loop in the `gardener-controller-manager` that signs
the CSRs during the initial TLS Bootstrapping also automatically signs
the CSR during a certificate rotation.

ℹ️ You can trigger an immediate renewal by annotating the `Secret` in the seed
cluster stated in the `.gardenClientConnection.kubeconfigSecret` field with
`gardener.cloud/operation=renew`. Within `10s`, gardenlet detects this and terminates
itself to request new credentials. After it has booted up again, gardenlet will issue a
new certificate independent of the remaining validity of the existing one.

ℹ️ Alternatively, annotate the respective `Seed` with `gardener.cloud/operation=renew-kubeconfig`.
This will make gardenlet annotate its own kubeconfig secret with `gardener.cloud/operation=renew`
and triggers the process described in the previous paragraph.

### Rotate Certificates Using Custom `kubeconfig`

When trying to rotate a custom certificate that wasn’t created by gardenlet
as part of the TLS Bootstrap, the x509 certificate's `Subject` field
needs to conform to the following:
  - the Common Name (CN) is prefixed with `gardener.cloud:system:seed:`
  - the Organization (O) equals `gardener.cloud:system:seeds`

Otherwise, the `gardener-controller-manager` doesn’t automatically
sign the CSR.
In this case, an external component or user needs to approve the CSR manually,
for example, using the command  `kubectl certificate approve  seed-csr-<...>`).
If that doesn’t happen within 15 minutes,
the gardenlet repeats the process and creates another CSR.

## Configuring the Seed to Work with gardenlet

The gardenlet works with a single seed, which must be configured in the
`GardenletConfiguration` under `.seedConfig`. This must be a copy of the
`Seed` resource, for example:

```yaml
apiVersion: gardenlet.config.gardener.cloud/v1alpha1
kind: GardenletConfiguration
seedConfig:
  metadata:
    name: my-seed
  spec:
    provider:
      type: aws
    # ...
    settings:
      scheduling:
        visible: true
```
(see [this yaml file](../../example/20-componentconfig-gardenlet.yaml) for a more complete example)

On startup, gardenlet registers a `Seed` resource using the given template
in the `seedConfig` if it's not present already.

## Component Configuration

In the component configuration for the gardenlet, it’s possible to define:

* settings for the Kubernetes clients interacting with the various clusters
* settings for the controllers inside the gardenlet
* settings for leader election and log levels, feature gates, and seed selection or seed configuration.

More information: [Example gardenlet Component Configuration](../../example/20-componentconfig-gardenlet.yaml).

## Heartbeats

Similar to how Kubernetes uses `Lease` objects for node heart beats
(see [KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/589-efficient-node-heartbeats/README.md)),
the gardenlet is using `Lease` objects for heart beats of the seed cluster.
Every two seconds, the gardenlet checks that the seed cluster's `/healthz`
endpoint returns HTTP status code 200.
If that is the case, the gardenlet renews the lease in the Garden cluster in the `gardener-system-seed-lease` namespace and updates
the `GardenletReady` condition in the `status.conditions` field of the `Seed` resource. For more information, see [this section](#lease-reconciler).

Similar to the `node-lifecycle-controller` inside the `kube-controller-manager`,
the `gardener-controller-manager` features a `seed-lifecycle-controller` that sets
the `GardenletReady` condition to `Unknown` in case the gardenlet fails to renew the lease.
As a consequence, the `gardener-scheduler` doesn’t consider this seed cluster for newly created shoot clusters anymore.

### `/healthz` Endpoint

The gardenlet includes an HTTP server that serves a `/healthz` endpoint.
It’s used as a liveness probe in the `Deployment` of the gardenlet.
If the gardenlet fails to renew its lease,
then the endpoint returns `500 Internal Server Error`, otherwise it returns `200 OK`.

Please note that the `/healthz` only indicates whether the gardenlet
could successfully probe the Seed's API server and renew the lease with
the Garden cluster.
It does *not* show that the Gardener extension API server (with the Gardener resource groups)
is available.
However, the gardenlet is designed to withstand such connection outages and
retries until the connection is reestablished.

## Controllers

The gardenlet consists out of several controllers which are now described in more detail.

### [`BackupBucket` Controller](../../pkg/gardenlet/controller/backupbucket)

The `BackupBucket` controller reconciles those `core.gardener.cloud/v1beta1.BackupBucket` resources whose `.spec.seedName` value is equal to the name of the `Seed` the respective `gardenlet` is responsible for.
A `core.gardener.cloud/v1beta1.BackupBucket` resource is created by the `Seed` controller if `.spec.backup` is defined in the `Seed`.

The controller adds finalizers to the `BackupBucket` and the secret mentioned in the `.spec.secretRef` of the `BackupBucket`. The controller also copies this secret to the seed cluster. Additionally, it creates an `extensions.gardener.cloud/v1alpha1.BackupBucket` resource (non-namespaced) in the seed cluster and waits until the responsible extension controller reconciles it (see [Contract: BackupBucket Resource](../extensions/resources/backupbucket.md) for more details).
The status from the reconciliation is reported in the `.status.lastOperation` field. Once the extension resource is ready and the `.status.generatedSecretRef` is set by the extension controller, the `gardenlet` copies the referenced secret to the `garden` namespace in the garden cluster. An owner reference to the `core.gardener.cloud/v1beta1.BackupBucket` is added to this secret.

If the `core.gardener.cloud/v1beta1.BackupBucket` is deleted, the controller deletes the generated secret in the garden cluster and the `extensions.gardener.cloud/v1alpha1.BackupBucket` resource in the seed cluster and it waits for the respective extension controller to remove its finalizers from the `extensions.gardener.cloud/v1alpha1.BackupBucket`. Then it deletes the secret in the seed cluster and finally removes the finalizers from the `core.gardener.cloud/v1beta1.BackupBucket` and the referred secret.

### [`BackupEntry` Controller](../../pkg/gardenlet/controller/backupentry)

The `BackupEntry` controller reconciles those `core.gardener.cloud/v1beta1.BackupEntry` resources whose `.spec.seedName` value is equal to the name of a `Seed` the respective gardenlet is responsible for.
Those resources are created by the `Shoot` controller (only if backup is enabled for the respective `Seed`) and there is exactly one `BackupEntry` per `Shoot`.

The controller creates an `extensions.gardener.cloud/v1alpha1.BackupEntry` resource (non-namespaced) in the seed cluster and waits until the responsible extension controller reconciled it (see [Contract: BackupEntry Resource](../extensions/resources/backupentry.md) for more details).
The status is populated in the `.status.lastOperation` field.

The `core.gardener.cloud/v1beta1.BackupEntry` resource has an owner reference pointing to the corresponding `Shoot`.
Hence, if the `Shoot` is deleted, the `BackupEntry` resource also gets deleted.
In this case, the controller deletes the `extensions.gardener.cloud/v1alpha1.BackupEntry` resource in the seed cluster and waits until the responsible extension controller has deleted it.
Afterwards, the finalizer of the `core.gardener.cloud/v1beta1.BackupEntry` resource is released so that it finally disappears from the system.

If the `spec.seedName` and `.status.seedName` of the `core.gardener.cloud/v1beta1.BackupEntry` are different, the controller will migrate it by annotating the `extensions.gardener.cloud/v1alpha1.BackupEntry` in the `Source Seed` with `gardener.cloud/operation: migrate`, waiting for it to be migrated successfully and eventually deleting it from the `Source Seed` cluster. Afterwards, the controller will recreate the `extensions.gardener.cloud/v1alpha1.BackupEntry` in the `Destination Seed`, annotate it with `gardener.cloud/operation: restore` and wait for the restore operation to finish. For more details about control plane migration, please read [Shoot Control Plane Migration](../operations/control_plane_migration.md#shoot-control-plane-migration).

##### Keep Backup for Deleted Shoots

In some scenarios it might be beneficial to not immediately delete the `BackupEntry`s (and with them, the etcd backup) for deleted `Shoot`s.

In this case you can configure the `.controllers.backupEntry.deletionGracePeriodHours` field in the component configuration of the gardenlet.
For example, if you set it to `48`, then the `BackupEntry`s for deleted `Shoot`s will only be deleted `48` hours after the `Shoot` was deleted.

Additionally, you can limit the [shoot purposes](../usage/shoot/shoot_purposes.md) for which this applies by setting `.controllers.backupEntry.deletionGracePeriodShootPurposes[]`.
For example, if you set it to `[production]` then only the `BackupEntry`s for `Shoot`s with `.spec.purpose=production` will be deleted after the configured grace period. All others will be deleted immediately after the `Shoot` deletion.

In case a `BackupEntry` is scheduled for future deletion but you want to delete it immediately, add the annotation `backupentry.core.gardener.cloud/force-deletion=true`.

### [`Bastion` Controller](../../pkg/gardenlet/controller/bastion)

The `Bastion` controller reconciles those `operations.gardener.cloud/v1alpha1.Bastion` resources whose `.spec.seedName` value is equal to the name of a `Seed` the respective gardenlet is responsible for.

The controller creates an `extensions.gardener.cloud/v1alpha1.Bastion` resource in the seed cluster in the shoot namespace with the same name as `operations.gardener.cloud/v1alpha1.Bastion`. Then it waits until the responsible extension controller has reconciled it (see [Contract: Bastion Resource](../extensions/resources/bastion.md) for more details). The status is populated in the `.status.conditions` and `.status.ingress` fields.

During the deletion of `operations.gardener.cloud/v1alpha1.Bastion` resources, the controller first sets the `Ready` condition to `False` and then deletes the `extensions.gardener.cloud/v1alpha1.Bastion` resource in the seed cluster.
Once this resource is gone, the finalizer of the `operations.gardener.cloud/v1alpha1.Bastion` resource is released, so it finally disappears from the system.

### [`ControllerInstallation` Controller](../../pkg/gardenlet/controller/controllerinstallation)

The `ControllerInstallation` controller in the `gardenlet` reconciles `ControllerInstallation` objects with the help of the following reconcilers.

#### ["Main" Reconciler](../../pkg/gardenlet/controller/controllerinstallation/controllerinstallation)

This reconciler is responsible for `ControllerInstallation`s referencing a `ControllerDeployment` whose `type=helm`.

For each `ControllerInstallation`, it creates a namespace on the seed cluster named `extension-<controller-installation-name>`.
Then, it creates a generic garden kubeconfig and garden access secret for the extension for [accessing the garden cluster](../extensions/garden-api-access.md).

After that, it unpacks the Helm chart tarball in the `ControllerDeployment`s `.providerConfig.chart` field and deploys the rendered resources to the seed cluster.
The Helm chart values in `.providerConfig.values` will be used and extended with some information about the Gardener environment and the seed cluster:

```yaml
gardener:
  version: <gardenlet-version>
  garden:
    clusterIdentity: <identity-of-garden-cluster>
    genericKubeconfigSecretName: <secret-name>
  gardenlet:
    featureGates:
      Foo: true
      Bar: false
      # ...
  seed:
    name: <seed-name>
    clusterIdentity: <identity-of-seed-cluster>
    annotations: <seed-annotations>
    labels: <seed-labels>
    spec: <seed-specification>
```

As of today, there are a few more fields in `.gardener.seed`, but it is recommended to use the `.gardener.seed.spec` if the Helm chart needs more information about the seed configuration.

The rendered chart will be deployed via a `ManagedResource` created in the `garden` namespace of the seed cluster.
It is labeled with `controllerinstallation-name=<name>` so that one can easily find the owning `ControllerInstallation` for an existing `ManagedResource`.

The reconciler maintains the `Installed` condition of the `ControllerInstallation` and sets it to `False` if the rendering or deployment fails.

#### ["Care" Reconciler](../../pkg/gardenlet/controller/controllerinstallation/care)

This reconciler reconciles `ControllerInstallation` objects and checks whether they are in a healthy state.
It checks the `.status.conditions` of the backing `ManagedResource` created in the `garden` namespace of the seed cluster.

- If the `ResourcesApplied` condition of the `ManagedResource` is `True`, then the `Installed` condition of the `ControllerInstallation` will be set to `True`.
- If the `ResourcesHealthy` condition of the `ManagedResource` is `True`, then the `Healthy` condition of the `ControllerInstallation` will be set to `True`.
- If the `ResourcesProgressing` condition of the `ManagedResource` is `True`, then the `Progressing` condition of the `ControllerInstallation` will be set to `True`.

A `ControllerInstallation` is considered "healthy" if `Applied=Healthy=True` and `Progressing=False`.

#### ["Required" Reconciler](../../pkg/gardenlet/controller/controllerinstallation/required)

This reconciler watches all resources in the `extensions.gardener.cloud` API group in the seed cluster.
It is responsible for maintaining the `Required` condition on `ControllerInstallation`s.
Concretely, when there is at least one extension resource in the seed cluster a `ControllerInstallation` is responsible for, then the status of the `Required` condition will be `True`.
If there are no extension resources anymore, its status will be `False`.

This condition is taken into account by the `ControllerRegistration` controller part of `gardener-controller-manager` when it computes which extensions have to be deployed to which seed cluster. See [Gardener Controller Manager](controller-manager.md#controllerregistration-controller) for more details.

### [`Gardenlet` Controller](../../pkg/gardenlet/controller/gardenlet)

The `Gardenlet` controller reconciles a `Gardenlet` resource with the same name as the `Seed` the gardenlet is responsible for.
This is used to implement self-upgrades of `gardenlet` based on information pulled from the garden cluster.
For a general overview, see [this document](../deployment/deploy_gardenlet.md).

On `Gardenlet` reconciliation, the controller deploys the `gardenlet` within its own cluster which after downloading the Helm chart specified in `.spec.deployment.helm.ociRepository` and rendering it with the provided values/configuration.

On `Gardenlet` deletion, nothing happens: The `gardenlet` does not terminate itself - deleting a `Gardenlet` object effectively means that self-upgrades are stopped.

### [`ManagedSeed` Controller](../../pkg/gardenlet/controller/managedseed)

The `ManagedSeed` controller in the `gardenlet` reconciles `ManagedSeed`s that refers to `Shoot` scheduled on `Seed` the gardenlet is responsible for.
Additionally, the controller monitors `Seed`s, which are owned by `ManagedSeed`s for which the gardenlet is responsible.

On `ManagedSeed` reconciliation, the controller first waits for the referenced `Shoot` to undergo a reconciliation process.
Once the `Shoot` is successfully reconciled, the controller sets the `ShootReconciled` status of the `ManagedSeed` to `true`.
Then, it creates `garden` namespace within the target shoot cluster.
The controller also manages secrets related to `Seed`s, such as the `backup` and `kubeconfig` secrets.
It ensures that these secrets are created and updated according to the `ManagedSeed` spec.
Finally, it deploys the `gardenlet` within the specified shoot cluster which registers the `Seed` cluster.

On `ManagedSeed` deletion, the controller first deletes the corresponding `Seed` that was originally created by the controller.
Subsequently, it deletes the `gardenlet` instance within the shoot cluster.
The controller also ensures the deletion of related `Seed` secrets.
Finally, the dedicated `garden` namespace within the shoot cluster is deleted.

### [`NetworkPolicy` Controller](../../pkg/gardenlet/controller/networkpolicy)

The `NetworkPolicy` controller reconciles `NetworkPolicy`s in all relevant namespaces in the seed cluster and provides so-called "general" policies for access to the runtime cluster's API server, DNS, public networks, etc.

The controller resolves the IP address of the Kubernetes service in the `default` namespace and creates an egress `NetworkPolicy`s for it.

For more details about `NetworkPolicy`s in Gardener, please see [`NetworkPolicy`s In Garden, Seed, Shoot Clusters](../operations/network_policies.md).

### [`Seed` Controller](../../pkg/gardenlet/controller/seed)

The `Seed` controller in the `gardenlet` reconciles `Seed` objects with the help of the following reconcilers.

#### ["Main Reconciler"](../../pkg/gardenlet/controller/seed/seed)

This reconciler is responsible for managing the seed's system components.
Those comprise CA certificates, the various `CustomResourceDefinition`s, the logging and monitoring stacks, and few central components like `gardener-resource-manager`, `etcd-druid`, `istio`, etc.

The reconciler also deploys a `BackupBucket` resource in the garden cluster in case the `Seed'`s `.spec.backup` is set.
It also checks whether the seed cluster's Kubernetes version is at least the [minimum supported version](../usage/shoot-operations/supported_k8s_versions.md#seed-cluster-versions) and errors in case this constraint is not met.

This reconciler maintains the `.status.lastOperation` field, i.e. it sets it:

- to `state=Progressing` before it executes its reconciliation flow.
- to `state=Error` in case an error occurs.
- to `state=Succeeded` in case the reconciliation succeeded.

#### ["Care" Reconciler](../../pkg/gardenlet/controller/seed/care)

This reconciler checks whether the seed system components (deployed by the "main" reconciler) are healthy.
It checks the `.status.conditions` of the backing `ManagedResource` created in the `garden` namespace of the seed cluster.
A `ManagedResource` is considered "healthy" if the conditions `ResourcesApplied=ResourcesHealthy=True` and `ResourcesProgressing=False`.

If all `ManagedResource`s are healthy, then the `SeedSystemComponentsHealthy` condition of the `Seed` will be set to `True`.
Otherwise, it will be set to `False`.

If at least one `ManagedResource` is unhealthy and there is threshold configuration for the conditions (in `.controllers.seedCare.conditionThresholds`), then the status of the `SeedSystemComponentsHealthy` condition will be set:

- to `Progressing` if it was `True` before.
- to `Progressing` if it was `Progressing` before and the `lastUpdateTime` of the condition does not exceed the configured threshold duration yet.
- to `False` if it was `Progressing` before and the `lastUpdateTime` of the condition exceeds the configured threshold duration.

The condition thresholds can be used to prevent reporting issues too early just because there is a rollout or a short disruption.
Only if the unhealthiness persists for at least the configured threshold duration, then the issues will be reported (by setting the status to `False`).

In order to compute the condition statuses, this reconciler considers `ManagedResource`s (in the `garden` and `istio-system` namespace) and their status, see [this document](resource-manager.md#conditions) for more information.
The following table explains which `ManagedResource`s are considered for which condition type:

| Condition Type                | `ManagedResource`s are considered when |
|-------------------------------|----------------------------------------|
| `SeedSystemComponentsHealthy` | `.spec.class` is set                   |

#### ["Lease" Reconciler](../../pkg/gardenlet/controller/seed/lease)

This reconciler checks whether the connection to the seed cluster's `/healthz` endpoint works.
If this succeeds, then it renews a `Lease` resource in the garden cluster's `gardener-system-seed-lease` namespace.
This indicates a heartbeat to the external world, and internally the `gardenlet` sets its health status to `true`.
In addition, the `GardenletReady` condition in the `status` of the `Seed` is set to `True`.
The whole process is similar to what the `kubelet` does to report heartbeats for its `Node` resource and its `KubeletReady` condition. For more information, see [this section](#heartbeats).

If the connection to the `/healthz` endpoint or the update of the `Lease` fails, then the internal health status of `gardenlet` is set to `false`.
Also, this internal health status is set to `false` automatically after some time, in case the controller gets stuck for whatever reason.
This internal health status is available via the `gardenlet`'s `/healthz` endpoint and is used for the `livenessProbe` in the `gardenlet` pod.

### [`Shoot` Controller](../../pkg/gardenlet/controller/shoot)

The `Shoot` controller in the `gardenlet` reconciles `Shoot` objects with the help of the following reconcilers.

#### ["Main" Reconciler](../../pkg/gardenlet/controller/shoot/shoot)

This reconciler is responsible for managing all shoot cluster components and implements the core logic for creating, updating, hibernating, deleting, and migrating shoot clusters.
It is also responsible for syncing the [`Cluster` cluster](../extensions/cluster.md) to the seed cluster before and after each successful shoot reconciliation.

The main reconciliation logic is performed in 3 different task flows dedicated to specific operation types:

- `reconcile` (operations: create, reconcile, restore): this is the main flow responsible for creation and regular reconciliation of shoots. Hibernating a shoot also triggers this flow. It is also used for restoration of the shoot control plane on the new seed (second half of a [Control Plane Migration](../operations/control_plane_migration.md#shoot-control-plane-migration))
- `migrate`: this flow is triggered when `spec.seedName` specifies a different seed than `status.seedName`. It performs the first half of the [Control Plane Migration](../operations/control_plane_migration.md#shoot-control-plane-migration), i.e., a backup (`migrate` operation) of all control plane components followed by a "shallow delete".
- `delete`: this flow is triggered when the shoot's `deletionTimestamp` is set, i.e., when it is deleted.

The gardenlet takes special care to prevent unnecessary shoot reconciliations.
This is important for several reasons, e.g., to not overload the seed API servers and to not exhaust infrastructure rate limits too fast.
The gardenlet performs shoot reconciliations according to the following rules:

- If `status.observedGeneration` is less than `metadata.generation`: this is the case, e.g., when the spec was changed, a [manual reconciliation operation](../usage/shoot-operations/shoot_operations.md) was triggered, or the shoot was deleted.
- If the [last operation](../usage/shoot/shoot_status.md) was not successful.
- If the shoot is in a [failed state](../usage/shoot/shoot_status.md), the gardenlet does not perform any reconciliation on the shoot (unless the retry operation was triggered). However, it syncs the `Cluster` resource to the seed in order to inform the extension controllers about the failed state.
- Regular reconciliations are performed with every `GardenletConfiguration.controllers.shoot.syncPeriod` (defaults to `1h`).
- Shoot reconciliations are not performed if the assigned seed cluster is not healthy or has not been reconciled by the current gardenlet version yet (determined by the `Seed.status.gardener` section). This is done to make sure that shoots are reconciled with fully rolled out seed system components after a Gardener upgrade. Otherwise, the gardenlet might perform operations of the new version that doesn't match the old version of the deployed seed system components, which might lead to unspecified behavior.

There are a few special cases that overwrite or confine how often and under which circumstances periodic shoot reconciliations are performed:

- In case the gardenlet config allows it (`controllers.shoot.respectSyncPeriodOverwrite`, disabled by default), the sync period for a shoot can be increased individually by setting the `shoot.gardener.cloud/sync-period` annotation. This is always allowed for shoots in the `garden` namespace. Shoots are not reconciled with a higher frequency than specified in `GardenletConfiguration.controllers.shoot.syncPeriod`.
- In case the gardenlet config allows it (`controllers.shoot.respectSyncPeriodOverwrite`, disabled by default), shoots can be marked as "ignored" by setting the `shoot.gardener.cloud/ignore` annotation. In this case, the gardenlet does not perform any reconciliation for the shoot.
- In case `GardenletConfiguration.controllers.shoot.reconcileInMaintenanceOnly` is enabled (disabled by default), the gardenlet performs regular shoot reconciliations only once in the respective maintenance time window (`GardenletConfiguration.controllers.shoot.syncPeriod` is ignored). The gardenlet randomly distributes shoot reconciliations over the maintenance time window to avoid high bursts of reconciliations (see [Shoot Maintenance](../usage/shoot/shoot_maintenance.md#cluster-reconciliation)).
- In case `Shoot.spec.maintenance.confineSpecUpdateRollout` is enabled (disabled by default), changes to the shoot specification are not rolled out immediately but only during the respective maintenance time window (see [Shoot Maintenance](../usage/shoot/shoot_maintenance.md)).

#### ["Care" Reconciler](../../pkg/gardenlet/controller/shoot/care)

This reconciler performs three "care" actions related to `Shoot`s.

##### Conditions

It maintains the following conditions:

- `APIServerAvailable`: The `/healthz` endpoint of the shoot's `kube-apiserver` is called and considered healthy when it responds with `200 OK`.
- `ControlPlaneHealthy`: The control plane is considered healthy when the respective `Deployment`s (for example `kube-apiserver`,`kube-controller-manager`), and `Etcd`s (for example `etcd-main`) exist and are healthy.
- `ObservabilityComponentsHealthy`: This condition is considered healthy when the respective `Deployment`s (for example `plutono`) and `StatefulSet`s (for example `prometheus`,`vali`) exist and are healthy.
- `EveryNodeReady`: The conditions of the worker nodes are checked (e.g., `Ready`, `MemoryPressure`). Also, it's checked whether the Kubernetes version of the installed `kubelet` matches the desired version specified in the `Shoot` resource.
- `SystemComponentsHealthy`: The conditions of the `ManagedResource`s are checked (e.g., `ResourcesApplied`). Also, it is verified whether the VPN tunnel connection is established (which is required for the `kube-apiserver` to communicate with the worker nodes).

Sometimes, `ManagedResource`s can have both `Healthy` and `Progressing` conditions set to `True` (e.g., when a `DaemonSet` rolls out one-by-one on a large cluster with many nodes) while this is not reflected in the `Shoot` status. In order to catch issues where the rollout gets stuck, one can set `.controllers.shootCare.managedResourceProgressingThreshold` in the `gardenlet`'s component configuration. If the `Progressing` condition is still `True` for more than the configured duration, the `SystemComponentsHealthy` condition in the `Shoot` is set to `False`, eventually.

Each condition can optionally also have error `codes` in order to indicate which type of issue was detected (see [Shoot Status](../usage/shoot/shoot_status.md) for more details).

Apart from the above, extension controllers can also contribute to the `status` or error `codes` of these conditions (see [Contributing to Shoot Health Status Conditions](../extensions/shoot-health-status-conditions.md) for more details).

If all checks for a certain conditions are succeeded, then its `status` will be set to `True`.
Otherwise, it will be set to `False`.

If at least one check fails and there is threshold configuration for the conditions (in `.controllers.seedCare.conditionThresholds`), then the status will be set:

- to `Progressing` if it was `True` before.
- to `Progressing` if it was `Progressing` before and the `lastUpdateTime` of the condition does not exceed the configured threshold duration yet.
- to `False` if it was `Progressing` before and the `lastUpdateTime` of the condition exceeds the configured threshold duration.

The condition thresholds can be used to prevent reporting issues too early just because there is a rollout or a short disruption.
Only if the unhealthiness persists for at least the configured threshold duration, then the issues will be reported (by setting the status to `False`).

Besides directly checking the status of `Deployment`s, `Etcd`s, `StatefulSet`s in the shoot namespace, this reconciler also considers `ManagedResource`s (in the shoot namespace) and their status in order to compute the condition statuses, see [this document](resource-manager.md#conditions) for more information.
The following table explains which `ManagedResource`s are considered for which condition type:

| Condition Type                   | `ManagedResource`s are considered when                                                                          |
|----------------------------------|-----------------------------------------------------------------------------------------------------------------|
| `ControlPlaneHealthy`            | `.spec.class=seed` and `care.gardener.cloud/condition-type` label either unset, or set to `ControlPlaneHealthy` |
| `ObservabilityComponentsHealthy` | `care.gardener.cloud/condition-type` label set to `ObservabilityComponentsHealthy`                              |
| `SystemComponentsHealthy`        | `.spec.class` unset or `care.gardener.cloud/condition-type` label set to `SystemComponentsHealthy`              |

##### Constraints And Automatic Webhook Remediation

Please see [Shoot Status](../usage/shoot/shoot_status.md#constraints) for more details.

##### Garbage Collection

Stale pods in the shoot namespace in the seed cluster and in the `kube-system` namespace in the shoot cluster are deleted.
A pod is considered stale when:

- it was terminated with reason `Evicted`.
- it was terminated with reason starting with `OutOf` (e.g., `OutOfCpu`).
- it was terminated with reason `NodeAffinity`.
- it is stuck in termination (i.e., if its `deletionTimestamp` is more than `5m` ago).

#### ["State" Reconciler](../../pkg/gardenlet/controller/shoot/state)

This reconciler periodically (default: every `6h`) performs backups of the state of `Shoot` clusters and persists them into `ShootState` resources into the same namespace as the `Shoot`s in the garden cluster.
It is only started in case the `gardenlet` is responsible for an unmanaged `Seed`, i.e. a `Seed` which is not backed by a `seedmanagement.gardener.cloud/v1alpha1.ManagedSeed` object.
Alternatively, it can be disabled by setting the `concurrentSyncs=0` for the controller in the `gardenlet`'s component configuration.

Please refer to [GEP-22: Improved Usage of the `ShootState` API](../proposals/22-improved-usage-of-shootstate-api.md) for all information.

### [`TokenRequestor` Controller For `ServiceAccount`s](../../pkg/controller/tokenrequestor)

The `gardenlet` uses an instance of the `TokenRequestor` controller which initially was developed in the context of the `gardener-resource-manager`, please read [this document](resource-manager.md#tokenrequestor-controller) for further information.

`gardenlet` uses it for requesting tokens for components running in the seed cluster that need to communicate with the garden cluster.
The mechanism works the same way as for shoot control plane components running in the seed which need to communicate with the shoot cluster.
However, `gardenlet`'s instance of the `TokenRequestor` controller is restricted to `Secret`s labeled with `resources.gardener.cloud/class=garden`.
Furthermore, it doesn't respect the `serviceaccount.resources.gardener.cloud/namespace` annotation. Instead, it always uses the seed's namespace in the garden cluster for managing `ServiceAccounts` and their tokens.

### [`TokenRequestor` Controller For `WorkloadIdentity`s](../../pkg/gardenlet/controller/tokenrequestor/workloadidentity)

The `TokenRequestorWorkloadIdentity` controller in the `gardenlet` reconciles `Secret`s labeled with `security.gardener.cloud/purpose=workload-identity-token-requestor`.
When it encounters such `Secret`, it associates the `Secret` with a specific `WorkloadIdentity` using the annotations `workloadidentity.security.gardener.cloud/name` and `workloadidentity.security.gardener.cloud/namespace`.
Any workload creating such `Secret`s is responsible to label and annotate the `Secret`s accordingly.
After the association is made, the `gardenlet` requests a token for the specific `WorkloadIdentity` from the Gardener API Server and writes it back in the `Secret`'s data against the `token` key.
The `gardenlet` is responsible to keep this token valid by refreshing it periodically.
The token is then used by components running in the seed cluster in order to present the said `WorkloadIdentity` before external systems, e.g. by calling cloud provider APIs.

Please refer to [GEP-26: Workload Identity - Trust Based Authentication](../proposals/26-workload-identity.md) for more details.

### [`VPAEvictionRequirements` Controller](../../pkg/gardenlet/controller/vpaevictionrequirements)

The `VPAEvictionRequirements` controller in the `gardenlet` reconciles `VerticalPodAutoscaler` objects labeled with `autoscaling.gardener.cloud/eviction-requirements: managed-by-controller`. It manages the [`EvictionRequirements`](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler/enhancements/4831-control-eviction-behavior) on a VPA object, which are used to restrict when and how a Pod can be evicted to apply a new resource recommendation.
Specifically, the following actions will be taken for the respective label and annotation configuration:
* If the VPA has the annotation `eviction-requirements.autoscaling.gardener.cloud/downscale-restriction: never`, an `EvictionRequirement` is added to the VPA object that allows evictions for upscaling only
* If the VPA has the annotation `eviction-requirements.autoscaling.gardener.cloud/downscale-restriction: in-maintenance-window-only`, the same `EvictionRequirement` is added to the VPA object when the Shoot is currently outside of its maintenance window. When the Shoot is inside its maintenance window, the `EvictionRequirement` is removed. Information about the Shoot maintenance window times are stored in the annotation `shoot.gardener.cloud/maintenance-window` on the VPA

## Managed Seeds

Gardener users can use shoot clusters as seed clusters, so-called "managed seeds" (aka "shooted seeds"),
by creating `ManagedSeed` resources.
By default, the gardenlet that manages this shoot cluster then automatically
creates a clone of itself with the same version and the same configuration
that it currently has.
Then it deploys the gardenlet clone into the managed seed cluster.

For more information, see [`ManagedSeed`s: Register Shoot as Seed](../operations/managed_seed.md).

## Migrating from Previous Gardener Versions

If your Gardener version doesn’t support gardenlets yet,
no special migration is required, but the following prerequisites must be met:

* Your Gardener version is at least 0.31 before upgrading to v1.
* You have to make sure that your garden cluster is exposed in a way
  that it’s reachable from all your seed clusters.

With previous Gardener versions, you had deployed the Gardener Helm chart
(incorporating the API server, `controller-manager`, and scheduler).
With v1, this stays the same, but you now have to deploy the gardenlet Helm chart as well
into all of your seeds (if they aren’t managed, as mentioned earlier).

See [Deploy a gardenlet](../deployment/deploy_gardenlet.md) for all instructions.

## Related Links

- [Gardener Architecture](architecture.md)
- [#356: Implement Gardener Scheduler](https://github.com/gardener/gardener/issues/356)
- [#2309: Add /healthz endpoint for gardenlet](https://github.com/gardener/gardener/pull/2309)
