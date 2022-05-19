# Project Namespace Access

## Service Account Manager
With Gardener `v1.47` a new role called `serviceaccountmanager` was introduced. This role allows to fully manage `serviceaccount`'s in the project namespace and request tokens for them. This is the preferred way of managing the access to a project namespace as it aims to replace the usage of the default `serviceaccount` secrets that will no longer be generated automatically with Kubernetes `v1.24+`.

### Create a Service Account
Once given the `serviceaccountmanager` role a user can create/update/delete `serviceaccount`s in the project namespace. In order to create a `serviceaccount` named "robot-user" run the following `kubectl` command:

```code
kubectl -n project-abc create sa robot-user
```

### Request a token for a Service Account
A token for the "robot-user" `serviceaccount` can be requested via the [TokenRequest API](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/).

The request can be made with `kubectl`
```bash
cat <<EOF | kubectl create -f - --raw /api/v1/namespaces/project-abc/serviceaccounts/robot-user/token
{
  "apiVersion": "authentication.k8s.io/v1",
  "kind": "TokenRequest",
  "spec": {
    "expirationSeconds": 3600
  }
}
EOF
```
or alternatively by directly calling the Kubernetes HTTP API
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

Mind that the returned token is not stored within the Kubernetes cluster, will be valid for `3600` seconds and will be invalidated if the "robot-user" `serviceaccount` is deleted.

### Delete a Service Account
In order to delete the `serviceaccount` named "robot-user" run the following `kubectl` command:

```code
kubectl -n project-abc delete sa robot-user
```

Mind that this will invalidate all existing tokens for the "robot-user" `serviceaccount`.
