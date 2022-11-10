# Supported Kubernetes Versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

The minimum version of the garden cluster that can be used to run Gardener is **`1.20.x`**.

## Seed cluster versions

The minimum version of a seed cluster that can be connected to Gardener is **`1.20.x`**.
Please note that Gardener does not support **`1.25`** seeds yet.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.20`** up to **`1.25`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.

> 👨🏼‍💻 Developers note: [This document](../development/new-kubernetes-version.md) explains what needs to be done in order to add support for a new Kubernetes version.
