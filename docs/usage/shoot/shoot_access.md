---
title: Accessing Shoot Clusters
---

# Accessing Shoot Clusters

After creation of a shoot cluster, end-users require a `kubeconfig` to access it. There are several options available to get to such `kubeconfig`.

## `shoots/adminkubeconfig` Subresource

The [`shoots/adminkubeconfig`](../../proposals/16-adminkubeconfig-subresource.md) subresource allows users to dynamically generate temporary `kubeconfig`s that can be used to access shoot cluster with `cluster-admin` privileges. The credentials associated with this `kubeconfig` are client certificates which have a very short validity and must be renewed before they expire (by calling the subresource endpoint again).

The username associated with such `kubeconfig` will be the same which is used for authenticating to the Gardener API. Apart from this advantage, the created `kubeconfig` will not be persisted anywhere.

In order to request such a `kubeconfig`, you can run the following commands (targeting the garden cluster):

```bash
export NAMESPACE=garden-my-namespace
export SHOOT_NAME=my-shoot
export KUBECONFIG=<kubeconfig for garden cluster>  # can be set using "gardenctl target --garden <landscape>"
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

In Python, you can use the native [`kubernetes` client](https://github.com/kubernetes-client/python) to create such a kubeconfig like this:

```python
# This script first loads an existing kubeconfig from your system, and then sends a request to the Gardener API to create a new kubeconfig for a shoot cluster. 
# The received kubeconfig is then decoded and a new API client is created for interacting with the shoot cluster.

import base64
import json
from kubernetes import client, config
import yaml

# Set configuration options
shoot_name="my-shoot" # Name of the shoot
project_namespace="garden-my-namespace" # Namespace of the project

# Load kubeconfig from default ~/.kube/config
config.load_kube_config()
api = client.ApiClient()

# Create kubeconfig request
kubeconfig_request = {
    'apiVersion': 'authentication.gardener.cloud/v1alpha1',
    'kind': 'AdminKubeconfigRequest',
    'spec': {
      'expirationSeconds': 600
    }
}

response = api.call_api(resource_path=f'/apis/core.gardener.cloud/v1beta1/namespaces/{project_namespace}/shoots/{shoot_name}/adminkubeconfig',
                        method='POST',
                        body=kubeconfig_request,
                        auth_settings=['BearerToken'],
                        _preload_content=False,
                        _return_http_data_only=True,
                       )

decoded_kubeconfig = base64.b64decode(json.loads(response.data)["status"]["kubeconfig"]).decode('utf-8')
print(decoded_kubeconfig)

# Create an API client to interact with the shoot cluster
shoot_api_client = config.new_client_from_config_dict(yaml.safe_load(decoded_kubeconfig))
v1 = client.CoreV1Api(shoot_api_client)
```

> **Note:** The [`gardenctl-v2`](https://github.com/gardener/gardenctl-v2) tool simplifies targeting shoot clusters. It automatically downloads a kubeconfig that uses the [gardenlogin](https://github.com/gardener/gardenlogin) kubectl auth plugin. This transparently manages authentication and certificate renewal without containing any credentials.

## `shoots/viewerkubeconfig` Subresource

The `shoots/viewerkubeconfig` subresource works similar to the [`shoots/adminkubeconfig`](#shootsadminkubeconfig-subresource).
The difference is that it returns a kubeconfig with read-only access for all APIs except the `core/v1.Secret` API and the resources which are specified in the `spec.kubernetes.kubeAPIServer.encryptionConfig` field in the Shoot (see [this document](../security/etcd_encryption_config.md)).

In order to request such a `kubeconfig`, you can run follow almost the same code as above - the only difference is that you need to use the `viewerkubeconfig` subresource.
For example, in bash this looks like this:

```bash
export NAMESPACE=garden-my-namespace
export SHOOT_NAME=my-shoot
kubectl create \
    -f <(printf '{"spec":{"expirationSeconds":600}}') \
    --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/viewerkubeconfig | \
    jq -r ".status.kubeconfig" | \
    base64 -d
```

The examples for other programming languages are similar to [the above](#shootsadminkubeconfig-subresource) and can be adapted accordingly.

> [!TIP]
> If the Gardener operator has configured a ["control plane wildcard certificate"](../../operations/trusted-tls-for-control-planes.md#register-a-trusted-wildcard-certificate), the issued kubeconfigs have a dedicated `Cluster` entry containing an endpoint that is served with this wildcard certificate.
> This could be a generally trusted certificate, e.g. from [Let's Encrypt](https://letsencrypt.org) or a similar certificate authority, i.e., it does not require you to specify the certificate authority bundle.
>
> ⚠️ This endpoint is specific to the seed cluster your `Shoot` is scheduled to, i.e., if the seed cluster changes (`.spec.seedName`, for example because of a [control plane migration](../../operations/control_plane_migration.md)), the endpoint changes as well. Have this in mind in case you consider using it!

## OpenID Connect

> **Note:** OpenID Connect is deprecated in favor of [Structured Authentication configuration](#structured-authentication). Setting OpenID Connect configurations is forbidden for clusters with Kubernetes version `>= 1.32`

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

It is the end-user's responsibility to incorporate the OpenID Connect configurations in the `kubeconfig` for accessing the cluster (i.e., Gardener will not automatically generate the `kubeconfig` based on these OIDC settings).
The recommended way is using the `kubectl` plugin called [`kubectl oidc-login`](https://github.com/int128/kubelogin) for OIDC authentication.

If you want to use the same OIDC configuration for all your shoots by default, then you can use the `ClusterOpenIDConnectPreset` and `OpenIDConnectPreset` API resources. They allow defaulting the `.spec.kubernetes.kubeAPIServer.oidcConfig` fields for newly created `Shoot`s such that you don't have to repeat yourself every time (similar to `PodPreset` resources in Kubernetes).
`ClusterOpenIDConnectPreset` specified OIDC configuration applies to `Projects` and `Shoots` cluster-wide (hence, only available to Gardener operators), while `OpenIDConnectPreset` is `Project`-scoped.
Shoots have to "opt-in" for such defaulting by using the `oidc=enable` label.

For further information on `(Cluster)OpenIDConnectPreset`, refer to [ClusterOpenIDConnectPreset and OpenIDConnectPreset](../security/openidconnect-presets.md).

For shoots with Kubernetes version `>= 1.30`, which have `StructuredAuthenticationConfiguration` feature gate enabled (enabled by default), it is advised to use Structured Authentication instead of configuring `.spec.kubernetes.kubeAPIServer.oidcConfig`.
If `oidcConfig` is configured, it is translated into an `AuthenticationConfiguration` file to use for [Structured Authentication configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration)

## Structured Authentication

For shoots with Kubernetes version `>= 1.30`, which have `StructuredAuthenticationConfiguration` feature gate enabled (enabled by default), `kube-apiserver` of shoot clusters can be provided with [Structured Authentication configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration) via the Shoot spec:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
...
spec:
  kubernetes:
    kubeAPIServer:
      structuredAuthentication:
        configMapName: name-of-configmap-containing-authentication-config
```

The `configMapName` references a user created `ConfigMap` in the project namespace containing the `AuthenticationConfiguration` in it's `config.yaml` data field.
Here is an example of such `ConfigMap`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: name-of-configmap-containing-authentication-config
  namespace: garden-my-project
data:
  config.yaml: |
    apiVersion: apiserver.config.k8s.io/v1beta1
    kind: AuthenticationConfiguration
    jwt:
    - issuer:
        url: https://issuer1.example.com
        audiences:
        - audience1
        - audience2
      claimMappings:
        username:
          expression: 'claims.username'
        groups:
          expression: 'claims.groups'
        uid:
          expression: 'claims.uid'
      claimValidationRules:
        expression: 'claims.hd == "example.com"'
        message: "the hosted domain name must be example.com"
```

The user is responsible for the validity of the configured `JWTAuthenticator`s.
Be aware that changing the configuration in the `ConfigMap` will be applied in the next `Shoot` reconciliation, but this is not automatically triggered.
If you want the changes to roll out immediately, [trigger a reconciliation explicitly](../shoot-operations/shoot_operations.md#immediate-reconciliation).

## Structured Authorization

For shoots with Kubernetes version `>= 1.30`, which have `StructuredAuthorizationConfiguration` feature gate enabled (enabled by default), `kube-apiserver` of shoot clusters can be provided with [Structured Authorization configuration](https://kubernetes.io/docs/reference/access-authn-authz/authorization/#using-configuration-file-for-authorization) via the Shoot spec:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
...
spec:
  kubernetes:
    kubeAPIServer:
      structuredAuthorization:
        configMapName: name-of-configmap-containing-authorization-config
        kubeconfigs:
        - authorizerName: my-webhook
          secretName: webhook-kubeconfig
```

The `configMapName` references a user created `ConfigMap` in the project namespace containing the `AuthorizationConfiguration` in it's `config.yaml` data field.
Here is an example of such `ConfigMap`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: name-of-configmap-containing-authorization-config
  namespace: garden-my-project
data:
  config.yaml: |
    apiVersion: apiserver.config.k8s.io/v1beta1
    kind: AuthorizationConfiguration
    authorizers:
    - type: Webhook
      name: my-webhook
      webhook:
        timeout: 3s
        subjectAccessReviewVersion: v1
        matchConditionSubjectAccessReviewVersion: v1
        failurePolicy: Deny
        matchConditions:
        - expression: request.resourceAttributes.namespace == 'kube-system'
```

In addition, it is required to provide a `Secret` for each authorizer.
This `Secret` should contain a kubeconfig with the server address of the webhook server, and optionally credentials for authentication:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: webhook-kubeconfig
  namespace: garden-my-project
data:
  kubeconfig: <base64-encoded-kubeconfig-for-authz-webhook>
```

The user is responsible for the validity of the configured authorizers.
Be aware that changing the configuration in the `ConfigMap` will be applied in the next `Shoot` reconciliation, but this is not automatically triggered.
If you want the changes to roll out immediately, [trigger a reconciliation explicitly](../shoot-operations/shoot_operations.md#immediate-reconciliation).

> [!NOTE]
> You can have one or more authorizers of type `Webhook` (no other types are supported).
>
> You are not allowed to specify the `authorizers[].webhook.connectionInfo` field.
> Instead, as mentioned above, provide a kubeconfig file containing the server address (and optionally, credentials that can be used by `kube-apiserver` in order to authenticate with the webhook server) by creating a `Secret` containing the kubeconfig (in the `.data.kubeconfig` key).
> Reference this `Secret` by adding it to `.spec.kubernetes.kubeAPIServer.structuredAuthorization.kubeconfigs[]` (choose the proper `authorizerName`, see example above).

Be aware of the fact that all webhook authorizers are added only after the `RBAC`/`Node` authorizers.
Hence, if RBAC already allows a request, your webhook authorizer might not get called.
