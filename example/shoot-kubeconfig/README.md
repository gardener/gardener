# Requesting admin kubeconfigs for a Shoot cluster

In order to request a kubeconfig using the [AdminKubeconfigRequest](../../docs/proposals/16-adminkubeconfig-subresource.md) subresource on `Shoot`, the `AdminKubeconfigRequest` feature gate must be enabled on the `gardener-apiserver`:

```text
--feature-gates AdminKubeconfigRequest=true
```

As support for subresource requests in `kubectl` is limited, the `--raw` option must be used:

```console
export NAMESPACE=my-namespace
export SHOOT_NAME=my-shoot
kubectl create \
    -f example/shoot-kubeconfig/kubeconfig-request.json \
    --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/adminkubeconfig | jq -r ".status.kubeconfig" | base64 -d
```
