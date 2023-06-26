---
title: Access to the Garden Cluster for Extensions
---

# Access to the Garden Cluster for Extensions

## Background

Historically, `gardenlet` has been the only component running in the seed cluster that has access to both the seed cluster and the garden cluster.
Accordingly, extensions running on the seed cluster didn't have access to the garden cluster.

Starting from Gardener [`v1.74.0`](https://github.com/gardener/gardener/releases/v1.74.0), there is a new mechanism for components running on seed clusters to get access to the garden cluster.
For this, `gardenlet` runs an instance of the [`TokenRequestor`](../concepts/gardenlet.md#tokenrequestor-controller) for requesting tokens that can be used to communicate with the garden cluster.

## Manually Requesting a Token for the Garden Cluster

Seed components that need to communicate with the garden cluster can request a token in the garden cluster by creating a garden access secret.
This secret has to be labelled with `resources.gardener.cloud/purpose=token-requestor` and `resources.gardener.cloud/class=garden`, e.g.:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: garden-access-example
  namespace: example
  labels:
    resources.gardener.cloud/purpose: token-requestor
    resources.gardener.cloud/class: garden
  annotations:
    serviceaccount.resources.gardener.cloud/name: example
type: Opaque
```

This will instruct gardenlet to create a new `ServiceAccount` named `example` in its own `seed-<seed-name>` namespace in the garden cluster, request a token for it, and populate the token in the secret's data under the `token` key.

> ⚠️ This feature is under development. The managed `ServiceAccounts` in the garden cluster don't have any API permissions as of now. They will be handled by the `SeedAuthorizer` in the future and equipped with permissions similar to the gardenlets' credentials. See [gardener/gardener#8001](https://github.com/gardener/gardener/issues/8001) for more information.

## Renewing All Garden Access Secrets

Operators can trigger an automatic renewal of all garden access secrets in a given `Seed` and their requested `ServiceAccount` tokens, e.g., when rotating the garden cluster's `ServiceAccount` signing key.
For this, the `Seed` has to be annotated with `gardener.cloud/operation=renew-garden-access-secrets`.
