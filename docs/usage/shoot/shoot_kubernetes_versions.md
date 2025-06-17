---
title: Shoot cluster supported Kubernetes versions and specifics
description: Defining the differences and requirements for upgrading to a supported Kubernetes version
---

# Shoot Kubernetes Minor Version Upgrades

Breaking changes may be introduced with new Kubernetes versions.
This documentation describes the differences and requirements for upgrading to a supported Kubernetes version.

## Upgrading to Kubernetes `v1.33`

- A new `deny-all` `NetworkPolicy` is deployed into the `kube-system` namespace of the `Shoot` cluster. `Shoot` owners that run workloads in the `kube-system` namespace are required to explicitly allow their expected `Ingress` and `Egress` traffic in `kube-system` via `NetworkPolicies`.

## Upgrading to Kubernetes `v1.32`

> [!TIP]
> It is recommended to [migrate from OIDC to `StructuredAuthentication`](shoot_access.md#migrating-from-oidc-to-structured-authentication-config) before updating to Kubernetes v1.32 in order to avoid not being able to revert the change.

- The `Shoot`'s `spec.kubernetes.kubeAPIServer.oidcConfig` field is forbidden.
- `Shoot` owners that have used `oidcConfig` are recommended to [configure `StructuredAuthentication`](shoot_access.md#migrating-from-oidc-to-structured-authentication-config). More information about `StructuredAuthentication` can be found in [Shoot Access](./shoot_access.md#structured-authentication).

## Upgrading to Kubernetes `v1.31`

- The `Shoot`'s `spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication` field is forbidden.
- The `Shoot`'s `.spec.kubernetes.kubelet.systemReserved` and `.spec.provider.workers[].kubernetes.kubelet.systemReserved` fields are forbidden. `Shoot` owners should use the `.spec.kubernetes.kubelet.kubeReserved` and `.spec.provider.workers[].kubernetes.kubelet.kubeReserved` fields.

## Upgrading to Kubernetes `v1.30`

- The `kubelet` `UnlimitedSwap` behavior, configured in the `Shoot`'s `.spec.{kubernetes,provider.workers[]}.kubelet.memorySwap.swapBehavior` fields, can no longer be used.
