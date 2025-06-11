---
title: Secret Binding to Credentials Binding Migration
---

# Secret Binding to Credentials Binding Migration

With the introduction of the [`CredentialsBinding`](../../api-reference/security.md) resource a new way of referencing credentials through the `Shoot` was created.
While `SecretBinding`s can only reference `Secret`s, `CredentialsBinding`s can also reference `WorkloadIdentity`s which provide an alternative authentication method.
`WorkloadIdentity`s do not directly contain credentials but are rather a representation of the workload that is going to access the user's account.

As `CredentialsBinding`s cover the functionality of `SecretBinding`s, the latter are considered legacy and will be deprecated in the future.
This incurs the need for migration from `SecretBinding` to `CredentialsBinding` resources.

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

2. Replace `secretBindingName` with `credentialsBindingName` in the `Shoot` spec. Mind that this migration will be allowed only if the old `SecretBinding` and the new `CredentialsBinding` refer to the same exact `Secret`. 

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
