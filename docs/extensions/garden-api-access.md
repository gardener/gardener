---
title: Access to the Garden Cluster for Extensions
---

# Access to the Garden Cluster for Extensions

## TL;DR

Extensions that are installed on seed clusters via a `ControllerInstallation` can simply read the kubeconfig file specified by the `GARDEN_KUBECONFIG` environment variable to create a garden cluster client.
With this, they use a short-lived token (valid for `12h`) for a dedicated `ServiceAccount` in the `seed-<seed-name>` namespace to securely access the garden cluster.

> ⚠️ This feature is under development. The managed `ServiceAccounts` in the garden cluster don't have any API permissions as of now. They will be handled by the `SeedAuthorizer` in the future and equipped with permissions similar to the gardenlets' credentials. See [gardener/gardener#8001](https://github.com/gardener/gardener/issues/8001) for more information.

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

## Using Gardenlet-Managed Garden Access

By default, extensions are equipped with secure access to the garden cluster using a dedicated `ServiceAccount` without requiring any additional action.
They can simply read the file specified by the `GARDEN_KUBECONFIG` and construct a garden client with it.

When installing a [`ControllerInstallation`](controllerregistration.md), gardenlet creates two secrets in the installation's namespace: a generic garden kubeconfig (`generic-garden-kubeconfig-<hash>`) and a garden access secret (`garden-access-extension`).
Additionally, it injects `volume`, `volumeMounts`, and two environment variables into all objects in the `apps` and `batch` API group:

- The `GARDEN_KUBECONFIG` environment variable points to the path where the generic garden kubeconfig is mounted.
  This in turn contains the garden cluster's address and current CA bundle as well as the path where the garden access token is mounted.
- The `SEED_NAME` environment variable is set to the name of the `Seed` where the extension is installed.
This is useful for restricting watches in the garden cluster to relevant objects.

For example, a `Deployment` deployed via a `ControllerInstallation` will be mutated as follows:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gardener-extension-provider-local
  annotations:
    reference.resources.gardener.cloud/secret-795f7ca6: garden-access-extension
    reference.resources.gardener.cloud/secret-d5f5a834: generic-garden-kubeconfig-81fb3a88
spec:
  template:
    metadata:
      annotations:
        reference.resources.gardener.cloud/secret-795f7ca6: garden-access-extension
        reference.resources.gardener.cloud/secret-d5f5a834: generic-garden-kubeconfig-81fb3a88
    spec:
      containers:
      - name: gardener-extension-provider-local
        env:
        - name: GARDEN_KUBECONFIG
          value: /var/run/secrets/gardener.cloud/garden/generic-kubeconfig/kubeconfig
        - name: SEED_NAME
          value: local
        volumeMounts:
        - mountPath: /var/run/secrets/gardener.cloud/garden/generic-kubeconfig
          name: garden-kubeconfig
          readOnly: true
      volumes:
      - name: garden-kubeconfig
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: kubeconfig
                path: kubeconfig
              name: generic-garden-kubeconfig-81fb3a88
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: garden-access-extension
              optional: false
```

The generic garden kubeconfig will look like this:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0t...
    server: https://garden.local.gardener.cloud:6443
  name: garden
contexts:
- context:
    cluster: garden
    user: extension
  name: garden
current-context: garden
users:
- name: extension
  user:
    tokenFile: /var/run/secrets/gardener.cloud/garden/generic-kubeconfig/token
```

## Using Self-Managed Garden Access

Extensions can also manage their own garden access secrets if required.
The default garden access secret is always created.
However, extensions can create additional garden access secrets as needed.
Note that these garden access secrets need to be mounted manually.
For this, gardenlet injects the name of the current generic garden kubeconfig secret into the chart's `.garden.genericKubeconfigSecretName` value.

When injecting `volume`, `volumeMounts`, and the `GARDEN_KUBECONFIG` environment variable, gardenlet skips all objects that already contain the `GARDEN_KUBECONFIG` environment variable.
It will still inject the default garden access into all other objects though.
The `SEED_NAME` environment variable is always injected.

## Renewing All Garden Access Secrets

Operators can trigger an automatic renewal of all garden access secrets in a given `Seed` and their requested `ServiceAccount` tokens, e.g., when rotating the garden cluster's `ServiceAccount` signing key.
For this, the `Seed` has to be annotated with `gardener.cloud/operation=renew-garden-access-secrets`.
