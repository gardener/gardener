# Supported Kubernetes versions

Currently, the Gardener supports the following Kubernetes versions:

## Garden cluster version

:warning: The minimum version of the garden cluster that can be used to run Gardener is **`1.10.x`**.
The reason for that is that the least supported Kubernetes version in Gardener is `1.10`.

## Seed cluster versions

:warning: The minimum version of a seed cluster that can be connected to Gardener is **`1.11.x`**.
The reason for that is that we require CRD status subresources for the extension controllers that we install into the seeds. They are enabled by default in `1.11`. Also, we install VPA as a part of controlplane component with version 0.5.0, which does not work on kubernetes version below `1.11`.

## Shoot cluster versions

| Cloud provider | Kubernetes 1.10 | Kubernetes 1.11 | Kubernetes 1.12 | Kubernetes 1.13 | Kubernetes 1.14 |
| -------------- | --------------- | --------------- | --------------- | --------------- | --------------- |
| AWS            | 1.10.0+         | 1.11.0+         | 1.12.1+         | 1.13.0+         | 1.14.0+         |
| Azure          | 1.10.1+         | 1.11.0+         | 1.12.1+         | 1.13.0+         | 1.14.0+         |
| GCP            | 1.10.0+         | 1.11.0+         | 1.12.1+         | 1.13.0+         | 1.14.0+         |
| OpenStack      | 1.10.1+         | 1.11.0+         | 1.12.1+         | 1.13.0+         | 1.14.0+         |
| Alicloud       | unsupported     | unsupported     | unsupported     | 1.13.0+         | 1.14.0+         |
