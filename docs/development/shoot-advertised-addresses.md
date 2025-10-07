# Shoot Advertised Addresses

Upon reconciliation, the `gardenlet` populates the list of advertised addresses
for the shoot cluster in the `.status.advertisedAddresses` field of the `Shoot`
resource.

This list provides endpoints for various services, such as the Kubernetes API
server of the shoot, the OIDC service account issuer, etc.

The example command below shows the list of advertised endpoints for a local
shoot cluster.

``` shell
$ kubectl --namespace garden-local get shoots local -o yaml | yq '.status.advertisedAddresses'
- name: external
  url: https://api.local.local.external.local.gardener.cloud
- name: internal
  url: https://api.local.local.internal.local.gardener.cloud
- name: service-account-issuer
  url: https://discovery.local.gardener.cloud/projects/local/shoots/41a0cdaa-6ad5-4846-9f6b-b7a7716538cb/issuer
```

The `external`, `internal` and `service-account-issuer` endpoints (amongst
others) are always present by default for a shoot cluster. Besides these,
additional endpoints from the shoot control-plane namespace may be advertised,
e.g. observability-related components such as `plutono`, `vali`, `prometheus`,
etc.

> [!NOTE]
> As of now, only `Ingress` resources support may be advertised using this label.
> In the future, support for `Gateway` resources will be added as well.

In order to advertise such endpoints, their respective `Ingress` resource needs
to be labeled with `endpoint.shoot.gardener.cloud/advertise=true`.

For example, if we want to advertise the `plutono` endpoint for our local shoot
cluster, we would label its respective `Ingress` resource like this:

``` shell
kubectl --namespace shoot--local--local \
    label ingress plutono endpoint.shoot.gardener.cloud/advertise=true
```

After successful reconciliation of the `Shoot` by the `gardenlet`, we should see a
new advertised endpoint for our cluster.

``` shell
- name: external
  url: https://api.local.local.external.local.gardener.cloud
- name: internal
  url: https://api.local.local.internal.local.gardener.cloud
- name: service-account-issuer
  url: https://discovery.local.gardener.cloud/projects/local/shoots/41a0cdaa-6ad5-4846-9f6b-b7a7716538cb/issuer
- name: ingress/plutono/0/0
  url: https://gu-local--local.ingress.local.seed.local.gardener.cloud
```
