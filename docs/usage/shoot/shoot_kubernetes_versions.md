---
title: Shoot cluster supported Kubernetes versions and specifics
description: Defining the differences and requirements for upgrading to a supported Kubernetes version
---

# Shoot Kubernetes Minor Version Upgrades

Breaking changes may be introduced with new Kubernetes versions.
This documentation describes the differences and requirements for upgrading to a supported Kubernetes version.

## Upgrading to Kubernetes `v1.33`

- A new `deny-all` `NetworkPolicy` is deployed into the `kube-system` namespace of the `Shoot` cluster. `Shoot` owners that run workloads in the `kube-system` namespace are required to explicitly allow their expected `Ingress` and `Egress` traffic in `kube-system` via `NetworkPolicies`.
- The new `ReduceDefaultCrashLoopBackOffDecay` feature gate was added to reduce both the initial delay and the maximum delay accrued between container restarts for a node for containers in `CrashLoopBackOff` across the cluster to the recommended values of 1s initial delay and 60s maximum delay. If you are also using the older feature gate `KubeletCrashLoopBackOffMax` with a configured per-node `CrashLoopBackOff.MaxContainerRestartPeriod`, the effective kubelet configuration will follow the conflict resolution policy described further in the documentation [here](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#reduced-container-restart-delay).

## Upgrading to Kubernetes `v1.32`

- The `Shoot`'s field `spec.kubernetes.kubeAPIServer.oidcConfig` is forbidden. `Shoot` owners that have used `oidcConfig` are recommended to configure `StructuredAuthentication`. More information about `StructuredAuthentication` can be found [here](./shoot_access.md#structured-authentication)

## Upgrading to Kubernetes `v1.31`

- The `Shoot`'s field `spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication` is forbidden.
- The `Shoot`'s fields `.spec.kubernetes.kubelet.systemReserved` and `.spec.provider.workers[].kubernetes.kubelet.systemReserved` are forbidden. `Shoot` owners should use the `.spec.kubernetes.kubelet.kubeReserved` and `.spec.provider.workers[].kubernetes.kubelet.kubeReserved` fields.

## Upgrading to Kubernetes `v1.30`

- The `kubelet` `UnlimitedSwap` behavior, configured in the `Shoot`'s `.spec.{kubernetes,provider.workers[]}.kubelet.memorySwap.swapBehavior` fields, can no longer be used.
