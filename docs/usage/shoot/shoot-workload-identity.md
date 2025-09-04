---
title: Shoot Workload Identity
description: Configure access to infrastructure accounts via workload identity instead of static credentials
---

# Shoot Workload Identity

[`WorkloadIdentity`](../../api-reference/security.md#workloadidentity) is a resource that allows workloads to be presented before external systems by giving them identities managed by Gardener.
As `WorkloadIdentity`s do not directly contain credentials we gain the ability to create `Shoot`s without the need of preliminary exchange of credentials.
For that to work users should establish trust to the Gardener Workload Identity Issuer in advance.
The issuer URL can be read from the [Gardener Info ConfigMap](../gardener/gardener_info_configmap.md).

> [!TIP]
> `Shoot`s that were previously using `Secret`s as authentication method can also be migrated to use `WorkloadIdentity`.
> As the `credentialsRef` field of [`CredentialsBinding`](../../api-reference/security.md#credentialsbinding) is immutable,
> one would have to create a new `CredentialsBinding` that references a `WorkloadIdentity` and set the `.spec.credentialsBindingName` field of the `Shoot`
> to refer to the newly created `CredentialsBinding`.

## JWT claims

The Gardener API server, as JWT issuer, sets the following standard claims as per [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519):

- **aud**: contains the audiences set in `WorkloadIdentity`'s `.spec.audiences` field.
- **iss**: issuer of the JWT, see above how to find its value for your Garden installation.
- **sub**: contains the subject of the JWT, it is following the format `gardener.cloud:workloadidentity:<workload-identity-namespace>:<workload-identity-name>:<workload-identity-uid>`, its value can be found also in `WorkloadIdentity`'s `.status.sub` field.
- **iat**: issued at time, a timestamp in the format of number of seconds since unix epoch.
- **exp**: expiration time, a timestamp in the format of number of seconds since unix epoch.
- **nbf**: not before time, a timestamp in the format of number of seconds since unix epoch.
- **jti**: JWT ID, a unique identifier for the JWT.

The Gardener API server is using the private namespace `gardener.cloud` to set additional claims in the JWT.
Their purpose is to bear Gardener specific information about the context of usage of the JWT.
The always set claim in this private namespace is `workloadIdentity` which contains the name, namespace and uid of the `WorkloadIdentity` resource used to issue the JWT.
When Gardenlet is requesting the JWT, the Gardener API server takes care to enhance the token with additional claims.
In the scenario where `WorkloadIdentity` is used as Shoot infrastructure credentials, such additional claims are:

- **shoot**: contains the name, namespace and uid of the Shoot using the JWT.
- **project**: contains the name and uid of the Gardener Project of the Shoot.
- **seed**: contains the name and uid of the Seed where the Shoot control plane is running.

Here is an example payload of workload identity JWT requested by Gardenlet:

```json
{
    "aud": "audience-1",
    "iss": "https://discovery.ingress.garden.local.gardener.cloud/garden/workload-identity/issuer",
    "sub": "gardener.cloud:workloadidentity:garden-bar:infra-a:3937d9b4-5b39-47b4-849b-ae25785834ca",
    "iat": 1756876768,
    "exp": 1756898368,
    "nbf": 1756876768,
    "jti": "5303b42e-7c75-439d-b7d3-3a842c7fdd98",
    "gardener.cloud": {
        "workloadIdentity": {
            "name": "infra-a",
            "namespace": "garden-bar",
            "uid": "3937d9b4-5b39-47b4-849b-ae25785834ca"
        },
        "shoot": {
            "name": "shoot-1",
            "namespace": "garden-dev",
            "uid": "a309d47b-4a30-4cc8-9371-5ef569e7c23e"
        },
        "project": {
            "name": "dev",
            "uid": "5660a988-506a-421b-b362-95f101629981"
        },
        "seed": {
            "name": "seed-1",
            "uid": "41ccaabf-5141-4189-9a14-0c3078beabcc"
        }
    }
}
```

## Infrastructure Providers

As of now `WorkloadIdentity` is supported for AWS, Azure and GCP. For detailed explanation on how to enable the feature, please consult the provider extension specific documentation:

- [provider-aws](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#aws-workload-identity-federation)
- [provider-azure](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#azure-workload-identity-federation)
- [provider-gcp](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#gcp-workload-identity-federation)
