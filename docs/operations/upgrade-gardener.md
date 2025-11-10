# Gardener Upgrade Guide

Upgrading Gardener affects the operator, the control plane, `gardenlet`s, managed seeds, shoots, and node operating system configurations.
A safe upgrade follows a predictable sequence and respects Gardener's reconciliation and rollout behavior.

## 1. Prepare Before Touching Anything

Familiarize yourself with the [version skew policy](../deployment/version_skew_policy.md).

Start by reading the [release notes](https://github.com/gardener/gardener/releases) of the target version you would like to deploy.
They describe breaking changes, new features, bug fixes and version increments.

Make sure to apply the breaking changes (if there are any) to your deployment manifests before increasing any version fields.
This prevents the Gardener components from starting with incompatible configuration.

### Deprecations and Backwards-Compatibility

Gardener's breaking change policy is conservative.
You can read all about it [here](../development/process.md#deprecations-and-backwards-compatibility).
Release notes clearly signal when manual migrations are required.

- Changes that affect the `Shoot` API surface are usually bundled with a Kubernetes minor version upgrade.
  For example, if a field may no longer be set, this usually takes only effect with a new Kubernetes minor version.
- Changes that affect operator-related APIs (those, end-users cannot write, like `Garden` or `Seed`) are usually deprecated first and only removed after three Gardener minor releases.
- Changes that affect extensions or their related APIs are usually deprecated first and only removed after nine Gardener minor versions.

## 2. Upgrade `gardener-operator` and Gardener Control Plane

Once your manifests match the target release, deploy the new `gardener-operator` [Helm chart](../../charts/gardener/operator).
After it comes up, it starts reconciling the `Garden` resource.
This also rolls out new versions of the Gardener control plane components (`gardener-apiserver`, `gardener-controller-manager`, etc.).

### Verify Readiness

Before continuing, wait for the `Garden` reconciliation to be successful and ensure that

- the `gardener.cloud/operation` annotation is no longer set.
- `.status.gardener.version` matches your target version.
- `.status.observedGeneration` matches `.metadata.generation`.
- `.status.lastOperation.state` is `Succeeded`

At this point, the `gardenlet`s still run the old version.
This is normal and expected/supported (similar to Kubernetes where the control plane is updated before the `kubelet`s).

Verify that the `.status.conditions[]` in the `Garden` resource (health checks) report status `True`.

## 3. Upgrade Your `gardenlet`s and Extensions

### Unmanaged Seeds

We start by upgrading the `gardenlet`s responsible for unmanaged seed clusters (i.e., `Seed`s which are not backed by a `Shoot`).
Unmanaged seed clusters should be represented by `Gardenlet` resources in the `garden` namespace (see [this](../deployment/deploy_gardenlet_manually.md#self-upgrades) for more information).

After a successful `Garden` reconciliation, `gardener-operator` automatically updates the `.spec.deployment.helm.ociRepository.ref` to its own version in all `Gardenlet` resources labeled with `operator.gardener.cloud/auto-update-gardenlet-helm-chart-ref=true`.
`gardenlet`s then update themselves via their "self-upgrade" functionality.

If you prefer to manage the `Gardenlet` resources via GitOps, Flux, or similar tools, then you should better manage the `.spec.deployment.helm.ociRepository.ref` field yourself and not label the resources as mentioned above (to prevent `gardener-operator` from interfering with your desired state).
In this case, make sure to now apply your `Gardenlet` resources (potentially containing a new version).

### Managed Seeds

When a `gardenlet` comes up, it starts reconciling `ManagedSeed` resources automatically.
No manual action from your side is required.
If the backing `Shoot` is currently reconciled, this must be finished first before the `ManagedSeed` reconciliation can be started.

Note that such reconciliations are jittered, so they will not converge instantly.
This protects the system from bursts of control-plane load.
The jitter period is `5m` by default and can be changed in `gardenlet`'s [component configuration](../../example/20-componentconfig-gardenlet.yaml) by setting `.controllers.managedSeed.syncJitterPeriod`.
You can set it to `0` if you don't want to have this behaviour (thus, speed up the rollouts of `gardenlet`s in `ManagedSeed`s), or increase this if you have a lot of seed clusters in your installation.

### Extensions

Extensions might be updated by bumping the version in their respective `ControllerDeployment` resource.
You can do this in parallel or after the `gardenlet` deployment — just make sure that compatibility is ensured by being aware of the extension's release notes.

### Verify Readiness

To ensure your seed clusters are updated (i.e., `gardenlet`s got rolled out everywhere), check your `Seed` resources and ensure that

- `.status.gardener.version` matches your target version. 

Verify that the `.status.conditions[]` in the `Seed` resource (health checks) report status `True`.

## 4. Shoot Reconciliations

By default, `Shoot`s are reconciled immediately after `gardenlet` comes up.
This might be fine for small Gardener installations, but usually, end-users don't expect such behaviour and rather want their clusters getting reconciled within a maintenance time window they specified.
You can read more about this [here](../usage/shoot/shoot_maintenance.md#cluster-reconciliation).
Change the behaviour by setting `.controllers.shoot.reconcileInMaintenanceOnly=true` in the `gardenlet` [component configuration](../../example/20-componentconfig-gardenlet.yaml).

If this is turned on, all `Shoot`s will be reconciled within the next 24 hours.

### Operating System Config Updates

Similar to how `ManagedSeed` reconciliations are jittered, the updates of the operating system config on worker nodes of shoot clusters is also jittered.
Such changes might involve `kubelet` restarts or other things, and in order to spread the load, nodes are not all updated in parallel.
By default, all nodes are updated within `5m`.
If you want to lower or increase this (minimum is `0` meaning all nodes are updated in parallel, maximum is `1800`) you can annotate your `Shoot`s with `shoot.gardener.cloud/cloud-config-execution-max-delay-seconds`.

## Image Vector and Overwrites

`gardener-operator`, `gardenlet`, and extensions have [image vectors](../../imagevector/containers.yaml) that describe the list of container images they deploy.
If you prefer to not use the images from the open-source repositories but rather replicate them into your own registry, please follow the instructions described [here](../deployment/image_vector.md).

Generally, each Gardener (and extension) release contains a GitHub release asset called `component-descriptor.yaml`.
You can look it up to examine the complete list of container images deployed by this component.
Use it to replicate them to your own registry and generate the overwrite as described in the document linked above.
