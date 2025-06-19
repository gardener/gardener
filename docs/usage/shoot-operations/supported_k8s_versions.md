---
title: Supported Kubernetes Versions
---

# Supported Kubernetes Versions

Currently, Gardener supports the following Kubernetes versions:

## Garden Clusters

The minimum version of a garden cluster that can be used to run Gardener is **`1.27.x`**.

## Seed Clusters

The minimum version of a seed cluster that can be connected to Gardener is **`1.27.x`**.

## Shoot Clusters

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.27`** up to **`1.33`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.

> 👨🏼‍💻 Developers note: The [Adding Support For a New Kubernetes Version](../../development/new-kubernetes-version.md) topic explains what needs to be done in order to add support for a new Kubernetes version.
