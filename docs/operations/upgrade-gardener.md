# Gardener Upgrade Guide

Upgrading Gardener is a multi-step process that updates its core components, including the operator, control plane, `gardenlet`s, and the clusters it manages (seeds and shoots). To ensure a smooth and safe upgrade, you should follow a specific sequence of steps. This process works with Gardener's built-in reconciliation and rollout mechanisms, which automatically apply changes across your landscape.

## 1. Prepare Before Touching Anything

First, understand Gardener's [version skew policy](../deployment/version_skew_policy.md), which defines which component versions are compatible with each other.

Next, carefully read the [release notes](https://github.com/gardener/gardener/releases) for your target version. The notes will detail any breaking changes, new features, and bug fixes.

If there are breaking changes, you must apply them to your configuration files (manifests) *before* you update the version numbers. This prevents components from starting with an incompatible configuration.

### Deprecations and Backwards-Compatibility

Gardener introduces breaking changes cautiously to ensure stability. You can read the full policy [here](../development/process.md#deprecations-and-backwards-compatibility). The release notes will always highlight when you need to perform manual steps.

- Changes affecting `Shoot` clusters are typically tied to a Kubernetes minor version upgrade.
- Changes to operator-level APIs (like `Garden` or `Seed`) are deprecated for at least three minor releases before being removed.
- For extensions, the deprecation period is even longer, typically nine minor releases.

## 2. Upgrade `gardener-operator` and Gardener Control Plane

After updating your configuration files, you can deploy the new `gardener-operator` using its [Helm chart](../../charts/gardener/operator). Once the new operator is running, it will automatically begin updating the `Garden` resource. This process rolls out the new versions of the Gardener control plane components, such as `gardener-apiserver` and `gardener-controller-manager`.

### Verify Readiness

Before moving to the next step, you must verify that the `Garden` resource has been successfully updated. Check the following:

- The `gardener.cloud/operation` annotation is removed, indicating the operation is complete.
- The `.status.gardener.version` field shows your target version.
- The `.status.observedGeneration` matches the `.metadata.generation`, meaning the latest configuration has been processed.
- The `.status.lastOperation.state` is `Succeeded`.

At this stage, it's normal for the `gardenlet`s to still be running the old version. This is similar to how Kubernetes upgrades its control plane before the `kubelet`s on worker nodes.

Finally, check the health conditions (`.status.conditions[]`) in the `Garden` resource to ensure they all report `True`.

## 3. Upgrade Your `gardenlet`s and Extensions

Next, upgrade the `gardenlet`s (and optionally your Gardener extensions).

### Unmanaged Seeds

Start with `gardenlet`s that manage "unmanaged" seeds - seed clusters that are not created via `Shoot`s. These should be configured using `Gardenlet` resources in the `garden` namespace.

If you've enabled auto-updates (by adding the `operator.gardener.cloud/auto-update-gardenlet-helm-chart-ref=true` label to your `Gardenlet` resources), the `gardener-operator` will automatically trigger the upgrade. The `gardenlet`s will then perform a self-upgrade.

Alternatively, if you manage your `Gardenlet` resources with a GitOps tool like Flux, you should not use the auto-update label. Instead, update the Helm chart reference (`.spec.deployment.helm.ociRepository.ref`) in your configuration and apply the changes yourself.

### Managed Seeds

Once a `gardenlet` is upgraded, it automatically begins upgrading any "managed" seeds it controls. No manual action is needed for this step. Note that a `ManagedSeed` upgrade will wait until any ongoing reconciliation of its underlying `Shoot` cluster is complete.

To prevent overloading the system, these upgrades are staggered using a "jitter" period, so they won't all start at once. By default, this period is 5 minutes. You can adjust it in the `gardenlet`'s [component configuration](../../example/20-componentconfig-gardenlet.yaml) by setting `.controllers.managedSeed.syncJitterPeriod`. Set it to `0` to start all upgrades immediately, or increase it if you have many seed clusters to manage the load.

### Extensions

You can upgrade extensions by updating the version in their `ControllerDeployment` resource. This can be done at the same time as the `gardenlet` upgrade, but always check the extension's release notes to ensure compatibility.

### Verify Readiness

To confirm that all your seed clusters are updated, check the following for each `Seed` resource:

- The `gardener.cloud/operation` annotation is removed, indicating the operation is complete.
- The `.status.gardener.version` field shows your target version.
- The `.status.observedGeneration` matches the `.metadata.generation`, meaning the latest configuration has been processed.
- The `.status.lastOperation.state` is `Succeeded`.

Finally, check the health conditions (`.status.conditions[]`) in the `Seed` resources to ensure they all report `True`.

## 4. Shoot Reconciliations

By default, after a `gardenlet` is upgraded, it immediately starts reconciling the `Shoot` clusters it manages. While this is suitable for small setups, it's often better to perform these reconciliations only during a predefined maintenance window.

To enable this, set `.controllers.shoot.reconcileInMaintenanceOnly=true` in the `gardenlet`'s [component configuration](../../example/20-componentconfig-gardenlet.yaml). When this setting is enabled, all `Shoot`s will be reconciled during their next scheduled maintenance window, which typically occurs within 24 hours. You can learn more about shoot maintenance [here](../usage/shoot/shoot_maintenance.md#cluster-reconciliation).

### Operating System Config Updates

Similar to seed upgrades, updates to the operating system on `Shoot` cluster worker nodes are also staggered. This prevents all nodes from being updated simultaneously, which could cause disruptions like `kubelet` restarts across the entire cluster.

By default, the rollout across all nodes completes within 5 minutes. You can customize this timeframe by adding the `shoot.gardener.cloud/cloud-config-execution-max-delay-seconds` annotation to your `Shoot`s. A value of `0` updates all nodes in parallel, while a higher value spreads the update over a longer period (up to 1800 seconds).

## Image Vector and Overwrites

Gardener components and extensions use an "image vector" to define the specific container images they deploy. If your organization requires using a private container registry, you can replicate the official images and configure Gardener to use them. Follow the instructions [here](../deployment/image_vector.md) to create an image vector overwrite.

Each Gardener release includes a `component-descriptor.yaml` file as a release asset. This file lists all container images for that version. You can use this list to pull the images, push them to your private registry, and generate the necessary configuration overwrite.
