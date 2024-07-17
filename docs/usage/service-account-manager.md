---
description: The role that allows a user to manage ServiceAccounts in the project namespace
weight: 14
---

# Service Account Manager

## Overview
With Gardener `v1.47`, a new role called `serviceaccountmanager` was introduced. This role allows to fully manage `ServiceAccount`'s in the project namespace and request tokens for them. This is the preferred way of managing the access to a project namespace, as it aims to replace the usage of the default `ServiceAccount` secrets that will no longer be generated automatically.

## Actions

Once assigned the `serviceaccountmanager` role, a user can create/update/delete `ServiceAccount`s in the project namespace.

### Create a Service Account
 In order to create a `ServiceAccount` named "robot-user", run the following `kubectl` command:

```code
kubectl -n project-abc create sa robot-user
```

### Request a Token for a Service Account
A token for the "robot-user" `ServiceAccount` can be requested via the [TokenRequest API](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/) in several ways:

```bash
kubectl -n project-abc create token robot-user --duration=3600s
```

- directly calling the Kubernetes HTTP API
```bash
curl -X POST https://api.gardener/api/v1/namespaces/project-abc/serviceaccounts/robot-user/token \
    -H "Authorization: Bearer <auth-token>" \
    -H "Content-Type: application/json" \
    -d '{
        "apiVersion": "authentication.k8s.io/v1",
        "kind": "TokenRequest",
        "spec": {
          "expirationSeconds": 3600
        }
      }'
```

Mind that the returned token is not stored within the Kubernetes cluster, will be valid for `3600` seconds, and will be invalidated if the "robot-user" `ServiceAccount` is deleted. Although `expirationSeconds` can be modified depending on the needs, the returned token's validity will not exceed the configured `service-account-max-token-expiration` duration for the garden cluster. It is advised that the actual `expirationTimestamp` is verified so that expectations are met. This can be done by asserting the `expirationTimestamp` in the `TokenRequestStatus` or the `exp` claim in the token itself.

### Delete a Service Account
In order to delete the `ServiceAccount` named "robot-user", run the following `kubectl` command:

```code
kubectl -n project-abc delete sa robot-user
```

This will invalidate all existing tokens for the "robot-user" `ServiceAccount`.