---
title: Control Plane Zone Pinning
description: How to pin the control plane of a shoot cluster to specific seed zones
---

# Control Plane Zone Pinning

By default, the gardenlet automatically selects availability zones for the control plane components of a shoot cluster based on the seed's zone configuration and the shoot's worker pool zones (see [Zone Selection](../../operations/seed_settings.md#zone-selection)).

For workerless shoots — which have no worker pools and therefore no worker zone hints — zone selection falls back to a random choice from the seed's available zones.

Operators can enable shoots to explicitly pin their control plane to specific seed zones by setting `spec.controlPlane.allowZonePinning: true` in the `CloudProfile` referenced by the shoot. This gate should only be enabled for providers where zone names are globally consistent across all users of that provider (i.e., `eu-west-1a` in one account refers to the same physical zone as `eu-west-1a` in another). Providers where zone names are local identifiers that differ per account must not enable this gate.

> [!NOTE]
> `spec.controlPlane.zones` is only evaluated at shoot creation time. The field is immutable once set, except that it can be expanded from 1 to 3 zones when upgrading the shoot's failure tolerance type to `zone` (see [Upgrading to Zone High Availability](#upgrading-to-zone-high-availability)).

> [!NOTE]
> Zone pinning applies to both workerless and regular shoot clusters. For shoots with worker pools, `spec.controlPlane.zones` controls where the control plane *pods* are scheduled on the seed cluster. This takes full precedence over the worker pool zones — the worker pool zones are not considered for control plane placement when `spec.controlPlane.zones` is set. Worker nodes themselves are still placed according to their pool's `spec.provider.workers[].zones`.

## Configuration

### Enabling Zone Pinning in the `CloudProfile`

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: my-cloudprofile
spec:
  controlPlane:
    allowZonePinning: true
  # ...
```

### Pinning the Shoot Control Plane

Once `allowZonePinning` is enabled in the `CloudProfile`, a shoot can specify its desired zones:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: garden-my-project
spec:
  cloudProfile:
    name: my-cloudprofile
  controlPlane:
    zones:
    - eu-west-1a
  # ...
```

The specified zone must be present in the seed's `.spec.provider.zones`. The admission webhook validates this at creation time.

For a highly available control plane with `failureTolerance.type: zone`, exactly three zones must be specified:

```yaml
spec:
  controlPlane:
    highAvailability:
      failureTolerance:
        type: zone
    zones:
    - eu-west-1a
    - eu-west-1b
    - eu-west-1c
```

## Upgrading to Zone High Availability

When upgrading a shoot's failure tolerance type to `zone`, the `spec.controlPlane.zones` field may be expanded from the original single zone to exactly three zones. The original zone must be retained in the new list; existing zones cannot be removed. All three zones must be present in the seed's `.spec.provider.zones`.

## Behavior

- **Seed selection**: The scheduler only considers seeds whose `.spec.provider.zones` contain all zones listed in `.spec.controlPlane.zones`. Seeds missing any of the requested zones are excluded regardless of the seed's zone selection mode (`Prefer`/`Enforce`).
- **Zone annotation**: The gardenlet sets the `high-availability-config.resources.gardener.cloud/zones` annotation on the seed namespace to exactly the specified zones (plus any zones already in use by existing persistent volumes, which cannot be changed without deleting and recreating the volumes).
- **Immutability**: `spec.controlPlane.zones` cannot be changed after the shoot is created, except for the zone-HA upgrade expansion described above.
- **CloudProfile gate revocation**: If `allowZonePinning` is later set to `false` in the `CloudProfile`, existing shoots that already have `spec.controlPlane.zones` set are not affected.
