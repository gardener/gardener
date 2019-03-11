# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.10.x`**.
The reason for that is that the least supported Kubernetes version in Gardener is `1.10`.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.10.x`**.
The reason for that is that we require CRD status subresources for the extension controllers that we install into the seeds.
CRD status subresources are alpha in `1.10` and can be enabled with the `CustomResourceSubresources` feature gate.
They are enabled by default in `1.11`. We allow `1.10` but users must make sure that the feature gate is enabled in this case.

## Shoot cluster versions

| Cloud provider | Kubernetes 1.10 | Kubernetes 1.11 | Kubernetes 1.12 | Kubernetes 1.13 |
| -------------- | --------------- | --------------- | --------------- | --------------- |
| AWS            | 1.10.0+         | 1.11.0+         | 1.12.1+         | 1.13.0+         |
| Azure          | 1.10.1+         | 1.11.0+         | 1.12.1+         | 1.13.0+         |
| GCP            | 1.10.0+         | 1.11.0+         | 1.12.1+         | 1.13.0+         |
| OpenStack      | 1.10.1+         | 1.11.0+         | 1.12.1+         | 1.13.0+         |
| Alicloud       | unsupported     | unsupported     | unsupported     | 1.13.0+         |
