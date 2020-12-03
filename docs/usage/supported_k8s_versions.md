# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.16.x`**.
The reason for that is that recent versions of the Kubernetes (extension) API server library is listing webhook configurations
in `admissionregistration.k8s.io/v1` version which is only served in Kubernetes clusters with version 1.16 and higher.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.11.x`**.
The reason for that is that we require CRD status subresources for the extension controllers that we install into the seeds. They are enabled by default in `1.11`. Also, we install VPA as a part of controlplane component with version 0.5.0, which does not work on kubernetes version below `1.11`.

> If `ManagedIstio` feature gate is enabled in gardenlet, the minimum version of a seed cluster is **`1.16.x`**. Additionally `TokenRequest` and `TokenRequestProjection` feature gates must be enabled and [Service Account Token Volume Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) set as well.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.10`** up to **`1.20`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.
