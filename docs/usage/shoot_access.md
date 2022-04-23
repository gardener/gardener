# Accessing Shoot Clusters

After creation of a shoot clusters, end-users requires a `kubeconfig` to access it. There are several options available to get to such `kubeconfig`.

## Static Token Kubeconfig :

This `kubeconfig` contains a [static token](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#static-token-file) and provides `cluster-admin` privileges.
It is created by default and persisted in the `<shoot-name>.kubeconfig` secret in the project namespace in the garden cluster.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
...
spec:
kubernetes:
    enableStaticTokenKubeconfig: true
...
```
It is **not** the recommended method to access the shoot cluster as the static token `kubeconfig` has some security flaws associated with it:
- The static token in the `kubeconfig` doesn't have any expiration date. To revoke the static token, the user needs to rotate the kuebcconfig credentials (see https://github.com/gardener/gardener/blob/master/docs/usage/shoot_operations.md#rotate-kubeconfig-credentials).
- The static token doesn't have any user identity associated with it. The user in that token will always be `system:cluster-admin` irrespective of the person accessing the cluster. Hence, it is impossible to audit the events in cluster.

## `shoots/adminkubeconfig` subresource

The [`shoots/adminkubeconfig`](https://github.com/gardener/gardener/blob/master/docs/proposals/16-adminkubeconfig-subresource.md) subresource allows users to dynamically generate short-lived `kubeconfig` that can be used to access shoot cluster with `cluster-admin` privileges. The credentials associated with this `kubeconfig` are client certificates which have a very short validity and must be renewed before they expire.

The username associated with such `kubeconfig` will be the same which is used for authenticating to the Gardener API. Apart from this advantage, the created `kubeconfig` will not be persisted anywhere.

```bash
export NAMESPACE=my-namespace
export SHOOT_NAME=my-shoot
kubectl create \
    -f <path>/<to>/kubeconfig-request.json \
    --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/adminkubeconfig | jq -r ".status.kubeconfig" | base64 -d
```

Here, the `kubeconfig-request.json` has the following content:

```json
{
    "apiVersion": "authentication.gardener.cloud/v1alpha1",
    "kind": "AdminKubeconfigRequest",
    "spec": {
        "expirationSeconds": 1000
    }
}
```
> The [`gardenctl-v2`](https://github.com/gardener/gardenctl-v2/) tool makes it easy to target shoot clusters and automatically renews such `kubeconfig` when required.

## `oidcConfig`

Gardener has `ClusterOpenIDConnectPreset` and `OpenIDConnectPreset` API resource for injecting additional runtime [OIDC](https://openid.net/connect/) requirements into a Shoot at creation time. The injected information contains configuration for the Kube API Server and optionally configuration for kubeconfig generation using said configuration. (Cluster)OpenIDConnectPreset contains the oidc server details like `clientID` , `issuerURL`. ClusterOpenIDConnectPreset specified oidc configuration applies to `Projects` and `Shoots` cluster-wide while OpenIDConnectPreset is `Project` scoped.

Gardener provides an admission controller (ClusterOpenIDConnectPreset and OpenIDConnectPreset) which, when enabled, applies ClusterOpenIDConnectPresets and OpenIDConnectPresets respectivly to incoming `Shoot` creation requests having label `oidc : enable` set. It mutates the shoot's `.spec.kubernetes.kubeAPIServer.oidcConfig` field with the oidc configuration which will be used for  `kubeconfig` generation.

It is the end-user responsibility to incorporate the OpenID Connect configurations in kubeconfig to access the cluster. One way is to use the kubectl plugin called [`kubectl oidc-login`](https://github.com/int128/kubelogin)  for OIDC authentication.

For further information on (Cluster)OpenIDConnectPreset, refer to [doc](https://github.com/gardener/gardener/blob/master/docs/usage/openidconnect-presets.md).
