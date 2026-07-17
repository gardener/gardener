# Live Control Plane Migration

> [!IMPORTANT]
> Live control plane migration ([GEP-39](https://github.com/gardener/enhancements/tree/main/geps/0039-live-control-plane-migration)) is a work in progress. Only the trigger and its prerequisites are wired up today; the gardenlet-side flow, retry, and abort semantics will be added in follow-up work. This document is intentionally minimal and will be enhanced along the way.

Live control plane migration moves a highly-available `Shoot` control plane from a *Source Seed* to a *Destination Seed* without the API-server downtime of [control plane migration](./control_plane_migration.md).

## Feature Gate

Live control plane migration is guarded by the `LiveControlPlaneMigration` feature gate in `gardener-apiserver`. The gate is currently in **alpha** and disabled by default. See [Feature Gates in Gardener](../deployment/feature_gates.md).

## Prerequisites

The `Shoot` and the involved `Seed`s must satisfy the following:

- The `Shoot` has `spec.controlPlane.highAvailability` configured.
- The `Shoot` is not hibernated and is not waking up (`spec.hibernation.enabled` and `.status.isHibernated` are both `false`).
- The `Source Seed` and `Destination Seed` use the same cloud provider type.
- The `Source Seed` and `Destination Seed` report the same gardenlet version.
- The [inter-region](#inter-region-distance) distance between the `Source Seed` and `Destination Seed` does not exceed the configured threshold. If the seeds are in the same region, this check is skipped.

### Inter-region distance

The distance check compares the `.spec.provider.region` of the two `Seed`s against a scheduler region `ConfigMap` in the `garden` namespace, labelled `scheduling.gardener.cloud/purpose: region-config` and annotated with `scheduling.gardener.cloud/cloudprofiles`. The `ConfigMap` maps each source region to the distances to other regions; the distance is an operator-defined metric (for example, network latency in milliseconds) used to decide how far apart two seeds may be. The default threshold is `180` (i.e. a maximum distance of 180 in the units used by the `ConfigMap`) and can be overridden per `ConfigMap` via `migration.gardener.cloud/inter-region-distance-threshold`.

To allow migration between distant regions for a specific `Shoot`, set `migration.gardener.cloud/allow-distant-regions=true` on the `Shoot`.

## Triggering the Migration

Both of the following are required on the `Shoot` to trigger a live control plane migration:

1. The intent annotation:

    ```yaml
    metadata:
      annotations:
        migration.gardener.cloud/live-migrate: "true"
    ```

2. A change of `.spec.seedName` to the `Destination Seed` via the [`shoots/binding`](../concepts/scheduler.md#shootsbinding-subresource) subresource:

    ```bash
    NAMESPACE=my-namespace
    SHOOT_NAME=my-shoot
    DEST_SEED_NAME=destination-seed

    kubectl get --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME} | jq -c '.spec.seedName = "'${DEST_SEED_NAME}'"' | kubectl replace --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/binding -f - | jq -r '.spec.seedName'
    ```
