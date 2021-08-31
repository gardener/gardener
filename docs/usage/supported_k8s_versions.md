# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.16.x`**.
The reason for that is that recent versions of the Kubernetes (extension) API server library is listing webhook configurations
in `admissionregistration.k8s.io/v1` version which is only served in Kubernetes clusters with version 1.16 and higher.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.18.x`**.
Kubernetes `1.18` sets the common ground for several Gardener features, e.g. `SeedKubeScheduler` ([ref](https://github.com/gardener/gardener/blob/master/docs/deployment/feature_gates.md#list-of-feature-gates)).
It also enables the Gardener code base to leverage more advanced Kubernetes features, like [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/), in the future.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.15`** up to **`1.22`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.
