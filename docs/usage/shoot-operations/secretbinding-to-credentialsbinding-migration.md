---
title: SecretBinding to CredentialsBinding Migration
---

# SecretBinding to CredentialsBinding Migration

With the introduction of the [`CredentialsBinding`](../../api-reference/security.md#credentialsbinding) resource a new way of referencing credentials through the `Shoot` was created.
While `SecretBinding`s can only reference `Secret`s, `CredentialsBinding`s can also reference `WorkloadIdentity`s which provide an alternative authentication method.
`WorkloadIdentity`s do not directly contain credentials but are rather a representation of the workload that is going to access the user's account.

As `CredentialsBinding`s cover the functionality of `SecretBinding`s, the latter are considered legacy and will be deprecated in the future.
This incurs the need for migration from `SecretBinding` to `CredentialsBinding` resources.

> [!NOTE]
> Mind that the migration will be allowed only if the old `SecretBinding` and the new `CredentialsBinding` refer to the same exact `Secret`.
> One cannot do a direct migration to a `CredentialsBinding` that reference a `WorkloadIdentity`.
> For information on how to use `WorkloadIdentity`, please refer to the following [document](../shoot/shoot-workload-identity.md).

## Migration Path

A standard use of `SecretBinding` can look like the following example.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: SecretBinding
metadata:
  name: infrastructure-credentials
  namespace: garden-proj
provider:
  type: foo-provider
secretRef:
  name: infrastructure-credentials-secret
  namespace: garden-proj
---
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: bar
  namespace: garden-proj
spec:
  secretBindingName: infrastructure-credentials
  ...
```

In order to migrate to `CredentialsBinding` one should:

1. Create a `CredentialsBinding` resource corresponding to the existing `SecretBinding`. The main difference is that we set the `kind` and `apiVersion` of the credentials that the `CredentialsBinding` is referencing.

    ```yaml
    apiVersion: security.gardener.cloud/v1alpha1
    kind: CredentialsBinding
    metadata:
      name: infrastructure-credentials
      namespace: garden-proj
    credentialsRef:
      apiVersion: v1
      kind: Secret
      name: infrastructure-credentials-secret
      namespace: garden-proj
    provider:
      type: foo-provider
    ```

1. Replace `secretBindingName` with `credentialsBindingName` in the `Shoot` spec.

    ```yaml
    apiVersion: core.gardener.cloud/v1beta1
    kind: Shoot
    metadata:
      name: bar
      namespace: garden-proj
    spec:
      credentialsBindingName: infrastructure-credentials
      ...
    ```
