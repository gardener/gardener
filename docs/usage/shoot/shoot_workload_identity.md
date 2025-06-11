---
title: Shoot Workload Identity
description: Configure access to infrastructure accounts via workload identity instead of static credentials
---

# Shoot Workload Identity

[`WorkloadIdentity`](../../api-reference/security.md) is resource that allows workloads to be presented before external systems
by giving them identities managed by the Gardener API server.
As `WorkloadIdentity`s do not directly contain credentials we gain the ability to create `Shoot`s without the need of preliminary exchange of credentials.
For that to work users should establish trust to the Gardener Workload Identity Issuer in advance.
The issuer URL can be read from the [Gardener Info ConfigMap](../gardener/gardener_info_configmap.md).

Gardener supports the following provider specific `WorkloadIdentity` implementations:
 - [AWS](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#aws-workload-identity-federation)
 - [Azure](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#azure-workload-identity-federation)
 - [GCP](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#gcp-workload-identity-federation)
