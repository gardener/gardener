# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.12.x`**.
The reason for that is that the `Gardenlet` is reporting readiness by using a `Lease` object supported by Kubernetes version later than `1.12`, same as the `Kubelet` is using `Leases` for the `Nodes`.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.11.x`**.
The reason for that is that we require CRD status subresources for the extension controllers that we install into the seeds. They are enabled by default in `1.11`. Also, we install VPA as a part of controlplane component with version 0.5.0, which does not work on kubernetes version below `1.11`.

> If `ManagedIstio` feature gate is enabled in gardenlet, the minimum version of a seed cluster is **`1.12.x`**. Additionally `TokenRequest` and `TokenRequestProjection` feature gates must be enabled and [Service Account Token Volume Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) set as well.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.10`** up to **`1.18`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.
