---
title: In-place Resource Updates
description: In-place updates of Pod resources
---

# In-place updates of Pod resources

This is a short guide covering the enablement of `in-place` resource updates in the `Vertical Pod Autoscaler`.

## Compatibility

`In-place` resource updates got introduced as an _alpha_ [feature](https://kubernetes.io/blog/2023/05/12/in-place-pod-resize-alpha/) with Kubernetes _1.27_ and got promoted to _beta_ with _1.33_.
On the `Vertical Pod Autoscaler` side, with Release [1.4.0](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.4.0), `in-place` resources updates are available as an _alpha_ [feature](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/features.md#in-place-updates-inplaceorrecreate) for `vpa-admission-controller` and `vpa-updater`.

### Kubernetes

With Kubernetes _1.33_ the `InPlacePodVerticalScaling` feature gate, enabling the `in-place` resources updates is enabled by default to all related components and __do not__ require additional intrumentation. Prior versions, from _1.27+_ require the `InPlacePodVerticalScaling` feature gate __to be enabled__ on both _kube-apiserver_ and _kubelet_.

### Vertical Pod Autoscaling

With [1.4.0](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.4.0), the `InPlaceOrRecreate` feature gate, enabling `in-place` for `vpa-admission-controller` and `vpa-updater`, got introduced as an _alpha_ feature. To explicitly enable the feature for `Shoot`, `Seed` or `Garden` clusters, the `Vertical Pod Autoscaling` configurations, for the respective cluster types, need to include an additional `InPlaceOrRecreate: true` entry in the `featureGates` map.

## Configuration

As described in the [compatibility](#compatibility) section, _alpha_ versions of the `InPlacePodVerticalScaling` Kubernetes feature require additional configuration to get the `in-place` updates enabled. This section covers the components that need to be configured both on `Kubernetes` and `Vertical Pod Autoscaler` sides.

### Shoot

Since `Vertical Pod Autoscaler` has it's `InPlaceOrRecreate` feature gate still in _alpha_, and Kubernetes promoted `InPlacePodVerticalScaling` to _beta_ just recently ( in _1.33_ ), we took the decision to allow configuring `in-place` resource updates only on `Shoot`(s) running _1.33+_.

- _Enable_ `InPlaceOrRecreate` feature gate in `Vertical Por Autoscaler` by updating:
  - `shoot.spec.kubernetes.verticalPodAutoscaler.featureGates` with:

      ```yaml
      InPlaceOrRecreate: true
      ```
- Make sure `InPlacePodVerticalScaling` Kubernetes feature gate is not explicitly disabled in:
  - `shoot.spec.kubernetes.kubeAPIServer.featureGates`
  - `shoot.spec.kubernetes.kubelet.featureGates`

### Seed

For `Seed` clusters, `Vertical Pod Autoscaler` features gates can be managed in `seed.spec.settings.verticalPodAutoscaler.featureGates`. There are no additional Kubernetes versions validation when configuring `Seed` clusters.

### Garden

For `Garden` clusters, `Vertical Pod Autoscaler` feature gates can be managed in `garden.spec.runtimeCluster.settings.verticalPodAutoscaler.featureGates`. There are no additional Kubernetes versions validation when configuring `Garden` clusters.

## References

- [Kubernetes v1.33: In-Place Pod Resize Graduated to Beta](https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/)
- [Kubernetes 1.27: In-place Resource Resize for Kubernetes Pods (alpha)](https://kubernetes.io/blog/2023/05/12/in-place-pod-resize-alpha/)
- [Vertical Pod Autoscaling Release 1.4.0](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.4.0)
- [Vertical Pod Autoscaling In-Place Updates](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/features.md#in-place-updates-inplaceorrecreate)
