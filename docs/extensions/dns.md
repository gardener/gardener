# Contract: `DNSProvider` and `DNSEntry` resources

Every shoot cluster requires external DNS records that are publicly resolvable.
The management of these DNS records requires provider-specific knowledge which is to be developed outside of the Gardener's core repository.

## What does Gardener create DNS records for?

### Internal domain name

Every shoot cluster's kube-apiserver running in the seed is exposed via a load balancer that has a public endpoint (IP or hostname).
This endpoint is used by end-users and also by system components (that are running in another network, e.g., the kubelet or kube-proxy) to talk to the cluster.
In order to be robust against changes of this endpoint (e.g., caused due to re-creation of the load balancer or move of the control plane to another seed cluster) Gardener creates a so-called *internal domain name* for every shoot cluster.
The *internal domain name* is a publicly resolvable DNS record that points to the load balancer of the kube-apiserver.
Gardener uses this domain name in the kubeconfigs of all system components (instead of writing the load balancer endpoint directly into it.
This way Gardener does not need to recreate all the kubeconfigs if the endpoint changes - it just needs to update the DNS record.

### External domain name

The internal domain name is not configurable by end-users directly but dictated by the Gardener administrator.
However, end-users usually prefer to have another DNS name, maybe even using their own domain sometimes to access their Kubernetes clusters.
Gardener supports that by creating another DNS record, named *external domain name*, that actually points to the *internal domain name*.
The kubeconfig handed out to end-users does contain this *external domain name*, i.e., users can access their clusters with the DNS name they like to.

As not every end-user has an own domain it is possible for Gardener administrators to configure so-called *default domains*.
If configured, shoots that do not specify a domain explicitly get an *external domain name* based on a default domain (unless explicitly stated that this shoot should not get an external domain name (`.spec.dns.provider=unmanaged`).

### Domain name for ingress (deprecated)

Gardener allows to deploy a `nginx-ingress-controller` into a shoot cluster (deprecated).
This controller is exposed via a public load balancer (again, either IP or hostname).
Gardener creates a wildcard DNS record pointing to this load balancer.
`Ingress` resources can later use this wildcard DNS record to expose underlying applications.

## What needs to be implemented to support a new DNS provider?

As part of the shoot flow Gardener will create two special resources in the seed cluster that need to be reconciled by an extension controller.
The first resource (`DNSProvider`) is a declaration of a DNS provider (e.g., `aws-route53`, `google-clouddns`, ...) with a reference to a `Secret` object that contains the provider-specific credentials in order to talk to the provider's API.
It also allows to specify two lists of domains that shall be allowed or disallowed to be used for DNS entries:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
  namespace: default
type: Opaque
data:
  # aws-route53 specific credentials here
---
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSProvider
metadata:
  name: my-aws-account
  namespace: default
spec:
  type: aws-route53
  secretRef:
    name: aws-credentials
  domains:
    include:
    - dev.my-fancy-domain.com
    exclude:
    - staging.my-fancy-domain.com
    - prod.my-fancy-domain.com
```

When reconciling this resource the DNS controller has to read information about available DNS zones to figure out which domains can actually be supported by the provided credentials.
Based on the constraints given in the `DNSProvider` resources `.spec.domains.{include|exclude}` fields it shall later only allow certain DNS entries.
Gardener waits until the `status` indicates that the registration went well:

```yaml
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSProvider
...
status:
  state: Ready
  message: everything ok
```

Other possible states are `Pending`, `Error`, and `Invalid`.
The DNS controller may provide an explanation of the `.status.state` in the `.status.message` field.

Now Gardener may create `DNSEntry` objects that represent the ask to create an actual external DNS record:

```yaml
---
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSEntry
metadata:
  name: dns
  namespace: default
spec:
  dnsName: apiserver.cluster1.dev.my-fancy-domain.com
  ttl: 600
  targets:
  - 8.8.8.8
```

It has to be automatically determined whether the to-be-created DNS record is of type `A` or `CNAME`.
The spec shall also allow the creation of `TXT` records, e.g.:

```yaml
---
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSEntry
metadata:
  name: dns
  namespace: default
spec:
  dnsName: data.apiserver.cluster1.dev.my-fancy-domain.com
  ttl: 120
  text: |
    content for the DNS TXT record
```

The `status` section of this resource looks similar like the `DNSProvider`'s.
Gardener is (as of today) only evaluating the `.status.state` and `.status.message` fields.

## References and additional resources

* [`DNSProvider` and `DNSEntry` API (Golang specification)](https://github.com/gardener/external-dns-management/tree/master/pkg/apis/dns/v1alpha1)
* [external-dns-management project in Gardener's GitHub organization](https://github.com/gardener/external-dns-management)
