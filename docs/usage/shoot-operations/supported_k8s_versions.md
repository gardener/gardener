---
title: Supported Kubernetes Versions
---

# Supported Kubernetes Versions

Currently, Gardener supports the following Kubernetes versions:

| Kubernetes version                                                         | Since Gardener version                                                   | Support added | Supported until |
|----------------------------------------------------------------------------|--------------------------------------------------------------------------|---------------|-----------------|
| [`v1.32`](https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/) | [`v1.113.0`](https://github.com/gardener/gardener/releases/tag/v1.113.0) | `2025-02-24`  | `2026-04-24`    |
| [`v1.33`](https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/) | [`v1.122.0`](https://github.com/gardener/gardener/releases/tag/v1.122.0) | `2025-06-27`  | `2026-08-27`    |
| [`v1.34`](https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/) | [`v1.132.0`](https://github.com/gardener/gardener/releases/tag/v1.132.0) | `2025-11-13`  | `2027-01-13`    |
| [`v1.35`](https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/) | [`v1.136.0`](https://github.com/gardener/gardener/releases/tag/v1.136.0) | `2026-02-14`  | `2027-04-14`    |

> [!NOTE]  
> Gardener supports Kubernetes versions for at least 14 months after their initial support date.

## Garden Clusters

The minimum version of a garden cluster that can be used to run Gardener is **`1.32.x`** up to **`1.35.x`**.

## Seed Clusters

The minimum version of a seed cluster that can be connected to Gardener is **`1.32.x`** up to **`1.35.x`**.

> [!WARNING]
> Kubernetes `v1.35.x` with `x` in `0..3` has a [regression](https://github.com/kubernetes/kubernetes/issues/137409) where the `MaxUnavailableStatefulSet` feature gate (enabled by default in `v1.35`) can cause StatefulSet rolling updates to deadlock when readiness probes fail.
> This affects seed clusters and can prevent components like `vpn-seed-server` from being updated.
> If you run a seed cluster with `v1.35.0` to `v1.35.3`, disable the feature gate in `kube-controller-manager` (`--feature-gates=MaxUnavailableStatefulSet=false`).
> This is fixed in `v1.35.4` and beyond.

## Shoot Clusters

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.32`** up to **`1.35`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.

> 👨🏼‍💻 Developers note: The [Adding Support For a New Kubernetes Version](../../development/new-kubernetes-version.md) topic explains what needs to be done in order to add support for a new Kubernetes version.
