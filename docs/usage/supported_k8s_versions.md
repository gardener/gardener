# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.10.x`**.
The reason for that is that the least supported Kubernetes version in Gardener is `1.10`.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.11.x`**.
The reason for that is that we require CRD status subresources for the extension controllers that we install into the seeds. They are enabled by default in `1.11`. Also, we install VPA as a part of controlplane component with version 0.5.0, which does not work on kubernetes version below `1.11`.

## Shoot cluster versions

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.10`** up to **`1.17`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.
