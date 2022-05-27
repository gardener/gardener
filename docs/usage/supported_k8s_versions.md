# Supported Kubernetes Versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.17.x`**.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.18.x`**.
Kubernetes `1.18` sets the common ground for several Gardener features, e.g. `SeedKubeScheduler` ([ref](https://github.com/gardener/gardener/blob/master/docs/deployment/feature_gates.md#list-of-feature-gates)).
It also enables the Gardener code base to leverage more advanced Kubernetes features, like [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/), in the future.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.17`** up to **`1.24`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.

> 👨🏼‍💻 Developers note: [This document](../development/new-kubernetes-version.md) explains what needs to be done in order to add support for a new Kubernetes version.
