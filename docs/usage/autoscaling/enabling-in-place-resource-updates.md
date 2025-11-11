---
title: Enabling In-Place Pod Resource Updates
description: Enablement of in-place Pod resource updates within Vertical Pod Autoscaler deployments
categories:
  - Operators
---

# Enabling In-Place Updates of Pod Resources

This is a short guide covering the adoption mechanism of `in-place` Pod resource updates in Gardener [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler) deployments.

## Compatibility

Refer to the [in-place resource updates](./in-place-resource-updates.md) guide for details on Kubernetes clusters compatibility, [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler) feature gate definition and availability.

## Configuration

Gardener provides a dedicated [resource manager](../../concepts/resource-manager.md) [webhook](../../concepts/resource-manager.md#webhooks) capable of _mutating_ VerticalPodAutoscaler resources, configured with [update mode](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/quickstart.md#contents) `Auto` or `Recreate`, with the `in-place` updates enabling `InPlaceOrRecreate`.
Available for deployment with both [gardenlet](../../concepts/gardenlet.md) and [gardener operator](../../concepts/operator.md), the _mutating_ webhook can be activated with the following feature gate, listed within the respective component manifest. Refer to the Gardener [feature gates](../../deployment/feature_gates.md) page for additional details:

```
VPAInPlaceUpdates
```

To keep a VerticalPodAutoscaler resource out of the _mutating_ webhook scope, add the following `skip` label, indicating that the resource should preserve its current configuration and __not__ get  updated:

```
vpa-in-place-updates.resources.gardener.cloud/skip
```

### Gardenlet

To enable the _mutating_ [resource manager](../../concepts/resource-manager.md) webhook, the `VPAInPlaceUpdates` feature gate must be set to `true`:

```yaml
featureGates:
  VPAInPlaceUpdates: true
```

Refer to the `gardenlet` component configuration [manifest](../../../example/20-componentconfig-gardenlet.yaml) for an overview.

#### Shoot

> When deployed in a `Shoot` cluster, the _mutating_ webhook targets `vertical pod autoscaler` resources __inside__ `kube-system` and `kubernetes-dashboard` namespaces.

To make use of the _mutating_ resource manager webhook, the `Shoot`'s [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler) deployment must have the `InPlaceOrRecreate` feature gate enabled. Follow the [in-place resource updates](./in-place-resource-updates.md#shoot) guide for more details about the Vertical Pod Autoscaler components setup.

#### Seed

>  When deployed in a `Seed` cluster, the _mutating_ webhook targets `vertical pod autoscaler` resources __outside__ `kube-system` and `kubernetes-dashboard` namespaces.

To make use of the _mutating_ resource manager webhook, the `Seed`'s [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler) deployment must have the `InPlaceOrRecreate` feature gate enabled. Follow the [in-place resource updates](./in-place-resource-updates.md#seed) guide for more details about the Vertical Pod Autoscaler components setup.

### Gardener Operator

To enable the _mutating_ [resource manager](../../concepts/resource-manager.md) webhook, the `VPAInPlaceUpdates` feature gate must be set to `true`:

```yaml
featureGates:
  VPAInPlaceUpdates: true
```

Refer to the `operator` component configuration [manifest](../../../example/operator/10-componentconfig.yaml) for an overview.

## References

- [Gardener Feature Gates](../../deployment/feature_gates.md)
- [Vertical Pod Autoscaling In-Place Updates](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/features.md#in-place-updates-inplaceorrecreate)
- [Vertical Pod Autoscaling In-Place Updates Guide](./in-place-resource-updates.md)
