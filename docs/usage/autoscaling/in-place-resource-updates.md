---
title: In-Place Resource Updates
description: In-place updates of Pod resources
categories:
  - Users
  - Operators
---

# In-Place Updates of Pod Resources

This is a short guide covering the enablement of `in-place` resource updates in the `Vertical Pod Autoscaler`.

## Compatibility

`In-place` resource updates got introduced as an alpha [feature](https://kubernetes.io/blog/2023/05/12/in-place-pod-resize-alpha/) in Kubernetes 1.27. In Kubernetes 1.33, it got promoted to beta and enabled by default.
On the `Vertical Pod Autoscaler` side, with Release [1.5.0](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.5.0), `in-place` resources updates are available as a _beta_ feature (enabled by default) for `vpa-admission-controller` and `vpa-updater`. For more details, see the [In-Place Updates documentation](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/features.md#in-place-updates-inplaceorrecreate).

### Kubernetes

With Kubernetes 1.33, the `InPlacePodVerticalScaling` feature gate, which enables `in-place` resource updates, is enabled by default and __does not__ require additional instrumentation. Prior versions, from Kubernetes 1.27 onwards require the `InPlacePodVerticalScaling` feature gate __to be enabled__ for both _kube-apiserver_ and _kubelet_.

### Vertical Pod Autoscaling

With [1.5.0](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.5.0), the `InPlaceOrRecreate` feature gate, which enables `in-place` resource updates for `vpa-admission-controller` and `vpa-updater`, got promoted to a __beta__ feature, making it _enabled_ by default.
Refer to the [usage guide](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/features.md#usage) for details about instrumenting a `VerticalPodAutoscaler` resource with `in-place` updates.

## Configuration

As described in the [Compatibility](#compatibility) section, _alpha_ versions of the `InPlacePodVerticalScaling` Kubernetes feature require additional configuration to get the `in-place` updates enabled. This section covers the components that need to be configured both on `Kubernetes` and `Vertical Pod Autoscaler` sides.

### Shoot

Since `Vertical Pod Autoscaler` has its `InPlaceOrRecreate` feature gate in _beta_, making it enabled by default, make sure that it is __not__ explicitly disabled. In addition, verify that the `InPlacePodVerticalScaling` Kubernetes feature gate is not explicitly disabled in the Shoot spec for:
- kube-apiserver: `.spec.kubernetes.kubeAPIServer.featureGates`
- kubelet: `.spec.kubernetes.kubelet.featureGates` or `.spec.provider.workers[].kubernetes.kubelet.featureGates`

### Seed

> **Disclaimer:** The following configurations are relevant for Gardener `Operators` that have access to `Seed` cluster(s).

For `Seed` clusters, `Vertical Pod Autoscaler` features gates can be managed in `seed.spec.settings.verticalPodAutoscaler.featureGates`. There are no additional Kubernetes versions validation when configuring `Seed` clusters.

### Garden

> **Disclaimer:** The following configurations are relevant for Gardener `Operators` that have access to `Garden` cluster(s).

For `Garden` clusters, `Vertical Pod Autoscaler` feature gates can be managed in `garden.spec.runtimeCluster.settings.verticalPodAutoscaler.featureGates`. There are no additional Kubernetes versions validation when configuring `Garden` clusters.

## References

- [Kubernetes v1.33: In-Place Pod Resize Graduated to Beta](https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/)
- [Kubernetes 1.27: In-place Resource Resize for Kubernetes Pods (alpha)](https://kubernetes.io/blog/2023/05/12/in-place-pod-resize-alpha/)
- [Vertical Pod Autoscaling Release 1.4.0](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.4.0)
- [Vertical Pod Autoscaling In-Place Updates](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/features.md#in-place-updates-inplaceorrecreate)
