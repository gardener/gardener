# Trusted TLS Certificate for Garden Runtime Cluster

In Garden Runtime Cluster components are exposed via `Ingress` resources, which make them addressable under the HTTPS protocol.

Examples:
- Plutono

Gardener generates the backing TLS certificates, which are signed by the shoot cluster's CA by default (self-signed).

Unlike with a self-contained Kubeconfig file, common internet browsers or operating systems don't trust a garden runtime's cluster CA and adding it as a trusted root is often undesired in enterprise environments.

Therefore, Gardener operators can predefine trusted wildcard certificates under which the mentioned endpoints will be served instead.

## Register a trusted wildcard certificate
Garden Runtime Cluster components are published under the ingress domain (`operator.gardener.cloud/v1alpha1.Garden.spec.runtimeCluster.ingress.domain`) a wildcard certificate is required.

For example:
- Garden Runtime cluster ingress domain: `dev.my-garden.example.com`
- `CN` or `SAN` for a certificate: `*.dev.my-garden.example.com`

It must be deployed as part of your landscape setup as a Kubernetes `Secret` inside the `garden` namespace of the garden runtime cluster.

Please ensure that the secret has the `gardener.cloud/role` label shown below:

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
  name: garden-ingress-certificate
  namespace: garden
type: Opaque
```

## Best Practice

While it is possible to create the wildcard certificates manually and deploy them to seed clusters, it is recommended to let certificate management components do this job. Often, a seed cluster is also a shoot cluster at the same time (ManagedSeed) and might already provide a certificate service extension.
Otherwise, a Gardener operator may use solutions like [Cert-Management](https://github.com/gardener/cert-management) or [Cert-Manager](https://github.com/jetstack/cert-manager).
