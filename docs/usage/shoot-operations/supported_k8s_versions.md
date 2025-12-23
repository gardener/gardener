---
title: Supported Kubernetes Versions
---

# Supported Kubernetes Versions

Currently, Gardener supports the following Kubernetes versions:

| Kubernetes version                                                         | Since Gardener version                                                   | Support added | Supported until |
|----------------------------------------------------------------------------|--------------------------------------------------------------------------|---------------|-----------------|
| [`v1.30`](https://kubernetes.io/blog/2024/04/17/kubernetes-v1-30-release/) | [`v1.95.0`](https://github.com/gardener/gardener/releases/tag/v1.95.0)   | `2024-05-16`  | `2025-07-16`    |
| [`v1.31`](https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/) | [`v1.106.0`](https://github.com/gardener/gardener/releases/tag/v1.106.0) | `2024-10-21`  | `2025-12-21`    |
| [`v1.32`](https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/) | [`v1.133.0`](https://github.com/gardener/gardener/releases/tag/v1.113.0) | `2025-02-24`  | `2026-04-24`    |
| [`v1.33`](https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/) | [`v1.122.0`](https://github.com/gardener/gardener/releases/tag/v1.122.0) | `2025-06-27`  | `2026-08-27`    |
| [`v1.34`](https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/) | [`v1.132.0`](https://github.com/gardener/gardener/releases/tag/v1.132.0) | `2025-11-13`  | `2027-01-13`    |

> [!NOTE]  
> Gardener supports Kubernetes versions for at least 14 months after their initial support date.

## Garden Clusters

The minimum version of a garden cluster that can be used to run Gardener is **`1.30.x`** up to **`1.34.x`**.

## Seed Clusters

The minimum version of a seed cluster that can be connected to Gardener is **`1.30.x`** up to **`1.34.x`**.

## Shoot Clusters

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.30`** up to **`1.34`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.

> 👨🏼‍💻 Developers note: The [Adding Support For a New Kubernetes Version](../../development/new-kubernetes-version.md) topic explains what needs to be done in order to add support for a new Kubernetes version.
