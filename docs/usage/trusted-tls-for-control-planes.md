# Trusted TLS certificate for shoot control planes
Shoot clusters are composed of several control plane components deployed by the Gardener and corresponding extensions.

Some components are exposed via `Ingress` resources which make them addressable under the HTTPS protocol.

Examples:
- Alertmanager
- Grafana for operators and end-users
- Prometheus

Gardener generates the backing TLS certificates which are signed by the shoot cluster's CA by default (self-signed).

Unlike with a self-contained Kubeconfig file, common internet browsers or operating systems don't trust a shoot's cluster CA and adding it as a trusted root is often undesired in enterprise environments.

Therefore, Gardener operators can predefine trusted wildcard certificates under which the mentioned endpoints will be served instead.

## Register a trusted wildcard certificate
Since control plane components are published under the ingress domain (`core.gardener.cloud/v1beta1.Seed.spec.dns.ingressDomain`) a wildcard certificate is required.

For example:
- Seed ingress domain: `dev.my-seed.example.com`
- `CN` or `SAN` for certificate: `*.dev.my-seed.example.com`

A wildcard certificate matches exactly one seed. It must be deployed as part of your landscape setup as a Kubernetes `Secret` inside the `garden` namespace of the corresponding seed cluster.

Please ensure that the secret has the `gardener.cloud/role` label shown below.

```yaml
apiVersion: v1
data:
  ca.crt: base64-encoded-ca.crt
  tls.crt: base64-encoded-tls.crt
  tls.key: base64-encoded-tls.key
kind: Secret
metadata:
  labels:
    gardener.cloud/role: controlplane-cert
  name: seed-ingress-certificate
  namespace: garden
type: Opaque
```

Gardener copies the secret during the reconciliation of shoot clusters to the shoot namespace in the seed. Afterwards, `Ingress` resources in that namespace for the mentioned components will refer to the wildcard certificate.

## Best practice
While it is possible to create the wildcard certificates manually and deploy them to seed clusters, it is recommended to let certificate management components do this job. Often, a seed cluster is also a shoot cluster at the same time (shooted seed) and might already provide a certificate service extension.
Otherwise, a Gardener operator may use solutions like [Cert-Management](https://github.com/gardener/cert-management) or [Cert-Manager](https://github.com/jetstack/cert-manager).