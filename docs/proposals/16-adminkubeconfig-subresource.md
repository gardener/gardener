---
title: 16 Dynamic kubeconfig generation for Shoot clusters
gep-number: 16
creation-date: 2021-04-02
status: implementable
authors:
- "@mvladev"
reviewers:
- "@rfranzke"
---

# GEP-16: Dynamic kubeconfig generation for Shoot clusters

## Table of Contents

- [GEP-16: Dynamic kubeconfig generation for Shoot clusters](#gep-16-dynamic-kubeconfig-generation-for-shoot-clusters)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
  - [Alternatives](#alternatives)

## Summary

This `GEP` introduces a new  `Shoot` subresource called `AdminKubeconfigRequest`, allowing for users to dynamically generate a short-lived `kubeconfig` that can be used to access the `Shoot` cluster as `cluster-admin`.

## Motivation

Today, when access to the created `Shoot` clusters is needed, a `kubeconfig` with static token credentials is used. This static token is in the `system:masters` group, granting it `cluster-admin` privileges. The `kubeconfig` is generated when the cluster is reconciled, stored in `ShootState`, and replicated in the `Project`'s namespace in a `Secret`. End-users can fetch the secret and use the `kubeconfig` inside it.

There are several problems with this approach:

- The token in the `kubeconfig` does not have any expiration, so end-users have to request a `kubeconfig` credential rotation if they want to revoke the token.
- There is no user identity in the token, e.g., if user `Joe` gets the `kubeconfig` from the `Secret`, the user in that token would be `system:cluster-admin` and not `Joe` when accessing the `Shoot` cluster with it. This makes auditing events in the cluster almost impossible.

### Goals

- Add a `Shoot` subresource called `adminkubeconfig` that would produce a `kubeconfig` used to access that `Shoot` cluster.
- The `kubeconfig` is not stored in the API Server, but generated for each request.
- In the `AdminKubeconfigRequest` send to that subresource, end-users can specify the expiration time of the credential.
- The identity (user) in the Gardener cluster would be part of the identity (x509 client certificate). E.g. if `Joe` authenticates against the Gardener API server, the generated certificate for `Shoot` authentication would have the following subject:

  - Common Name: `Joe`
  - Organisation: `system:masters`

- The maximum validity of the certificate can be enforced by setting a flag on the `gardener-apiserver`.
- Deprecate and remove the old `{shoot-name}.kubeconfig` secrets in each `Project` namespace.

### Non-Goals

- Generate `OpenID Connect` kubeconfigs

## Proposal

The `gardener-apiserver` would serve a new `shoots/adminkubeconfig` resource. It can only accept `CREATE` calls and accept `AdminKubeconfigRequest`. An `AdminKubeconfigRequest` would have the following structure:

```yaml
apiVersion: authentication.gardener.cloud/v1alpha1
kind: AdminKubeconfigRequest
spec:
  expirationSeconds: 3600
```

Where `expirationSeconds` is the validity of the certificate in seconds. In this case, it would be `1 hour`. The maximum validity of a `AdminKubeconfigRequest` is configured by the `--shoot-admin-kubeconfig-max-expiration` flag in the `gardener-apiserver`.

When such request is received, the API server would find the `ShootState` associated with that cluster and generate a `kubeconfig`. The x509 client certificate would be signed by the `Shoot` cluster's CA and the user used in the subject's common name would be from the `User.Info` used to make the request.

```yaml
apiVersion: authentication.gardener.cloud/v1alpha1
kind: AdminKubeconfigRequest
spec:
  expirationSeconds: 3600
status:
  expirationTimestamp: "2021-02-22T09:06:51Z"
  kubeConfig: # this is normally base64-encoded, but decoded for the example
    apiVersion: v1
    clusters:
    - cluster:
        certificate-authority-data: LS0tLS1....
        server: https://api.shoot-cluster
      name: shoot-cluster-a
    contexts:
    - context:
        cluster: shoot-cluster-a
        user: shoot-cluster-a
      name: shoot-cluster-a
    current-context: shoot-cluster-a
    kind: Config
    preferences: {}
    users:
    - name: shoot-cluster-a
      user:
        client-certificate-data: LS0tLS1CRUd...
        client-key-data: LS0tLS1CRUd...
```

New feature gate called `AdminKubeconfigRequest` enables the above mentioned API in the `gardener-apiserver`. The old `{shoot-name}.kubeconfig` is kept, but deprecated and will be removed in the future.

In order to get the server's address used in the `kubeconfig`, the Shoot's `status` should be updated with new entries:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: crazy-botany
  namespace: garden-dev
spec: {}
status:
  advertisedAddresses:
  - name: external
    url: https://api.shoot-cluster.external.foo
  - name: internal
    url: https://api.shoot-cluster.internal.foo
  - name: ip
    url: https://1.2.3.4
```

This is needed, because the Gardener API server might not know on which IP address the API server is advertised on (e.g. DNS is disabled).

If there are multiple entries, each would be added in a separate `cluster` in the `kubeconfig` and a `context` with the same name would be added as well. The current context would be selected as the first entry in the `advertisedAddresses` list (`.status.advertisedAddresses[0]`).

## Alternatives

- [Dynamic OpenID Connect Webhook Authenticator](https://github.com/gardener/oidc-webhook-authenticator) can be used instead. Ideally, cluster admins can enable either or both.
