# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.16.x`**.
The reason for that is that recent versions of the Kubernetes (extension) API server library is listing webhook configurations
in `admissionregistration.k8s.io/v1` version which is only served in Kubernetes clusters with version 1.16 and higher.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.15.x`**.

> If `ManagedIstio` feature gate is enabled in gardenlet, the minimum version of a seed cluster is **`1.16.x`**. Additionally `TokenRequest` and `TokenRequestProjection` feature gates must be enabled and [Service Account Token Volume Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) set as well.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.15`** up to **`1.21`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.
