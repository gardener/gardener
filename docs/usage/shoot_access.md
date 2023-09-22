# Accessing Shoot Clusters

After creation of a shoot cluster, end-users require a `kubeconfig` to access it. There are several options available to get to such `kubeconfig`.

## `shoots/adminkubeconfig` Subresource

The [`shoots/adminkubeconfig`](../proposals/16-adminkubeconfig-subresource.md) subresource allows users to dynamically generate temporary `kubeconfig`s that can be used to access shoot cluster with `cluster-admin` privileges. The credentials associated with this `kubeconfig` are client certificates which have a very short validity and must be renewed before they expire (by calling the subresource endpoint again).

The username associated with such `kubeconfig` will be the same which is used for authenticating to the Gardener API. Apart from this advantage, the created `kubeconfig` will not be persisted anywhere.

In order to request such a `kubeconfig`, you can run the following commands:

```bash
export NAMESPACE=my-namespace
export SHOOT_NAME=my-shoot
kubectl create \
    -f <(printf '{"spec":{"expirationSeconds":600}}') \
    --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/adminkubeconfig | \
    jq -r ".status.kubeconfig" | \
    base64 -d
```


You also can use controller-runtime `client` (>= v0.14.3) to create such a kubeconfig from your go code like so:

```go
expiration := 10 * time.Minute
expirationSeconds := int64(expiration.Seconds())
adminKubeconfigRequest := &authenticationv1alpha1.AdminKubeconfigRequest{
  Spec: authenticationv1alpha1.AdminKubeconfigRequestSpec{
    ExpirationSeconds: &expirationSeconds,
  },
}
err := client.SubResource("adminkubeconfig").Create(ctx, shoot, adminKubeconfigRequest)
if err != nil {
  return err
}
config = adminKubeconfigRequest.Status.Kubeconfig
```

> **Note:** The [`gardenctl-v2`](https://github.com/gardener/gardenctl-v2) tool simplifies targeting shoot clusters. It automatically downloads a kubeconfig that uses the [gardenlogin](https://github.com/gardener/gardenlogin) kubectl auth plugin. This transparently manages authentication and certificate renewal without containing any credentials.

## OpenID Connect

The `kube-apiserver` of shoot clusters can be provided with [OpenID Connect configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens) via the Shoot spec:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
...
spec:
  kubernetes:
    oidcConfig:
      ...
```

It is the end-user's responsibility to incorporate the OpenID Connect configurations in `kubeconfig` for accessing the cluster (i.e., Gardener will not automatically generate `kubeconfig` based on these OIDC settings).
The recommended way is using the `kubectl` plugin called [`kubectl oidc-login`](https://github.com/int128/kubelogin) for OIDC authentication.

If you want to use the same OIDC configuration for all your shoots by default, then you can use the `ClusterOpenIDConnectPreset` and `OpenIDConnectPreset` API resources. They allow defaulting the `.spec.kubernetes.kubeAPIServer.oidcConfig` fields for newly created `Shoot`s such that you don't have to repeat yourself every time (similar to `PodPreset` resources in Kubernetes).
`ClusterOpenIDConnectPreset` specified OIDC configuration applies to `Projects` and `Shoots` cluster-wide (hence, only available to Gardener operators) while `OpenIDConnectPreset` is `Project`-scoped.
Shoots have to "opt-in" for such defaulting by using the `oidc=enable` label.

For further information on `(Cluster)OpenIDConnectPreset`, refer to [ClusterOpenIDConnectPreset and OpenIDConnectPreset](openidconnect-presets.md).

## Static Token kubeconfig

> **Note:** Static token kubeconfig is not available for Shoot clusters using Kubernetes version >= 1.27. The [`shoots/adminkubeconfig` subresource](#shootsadminkubeconfig-subresource) should be used instead.

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

It is **not** the recommended method to access the shoot cluster, as the static token `kubeconfig` has some security flaws associated with it:
- The static token in the `kubeconfig` doesn't have any expiration date. Read [this document](shoot_credentials_rotation.md#kubeconfig) to learn how to rotate the static token.
- The static token doesn't have any user identity associated with it. The user in that token will always be `system:cluster-admin`, irrespective of the person accessing the cluster. Hence, it is impossible to audit the events in cluster.

When `enableStaticTokenKubeconfig` field is not explicitly set in the Shoot spec:
- for Shoot clusters using Kubernetes version < 1.26 the field is defaulted to `true`.
- for Shoot clusters using Kubernetes version >= 1.26 the field is defaulted to `false`.

> **Note:** Starting with Kubernetes 1.27, the `enableStaticTokenKubeconfig` field will be locked to `false`. 
