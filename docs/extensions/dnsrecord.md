# Contract: `DNSRecord` resources

Every shoot cluster requires external DNS records that are publicly resolvable.
The management of these DNS records requires provider-specific knowledge which is to be developed outside the Gardener's core repository.

Currently, Gardener uses [`DNSProvider` and `DNSEntry` resources](dns.md). However, this introduces undesired coupling of Gardener to a controller that does not adhere to the Gardener extension contracts. Because of this, we plan to stop using `DNSProvider` and `DNSEntry` resources for Gardener DNS records in the future and use the `DNSRecord` resources described here instead. 

## What does Gardener create DNS records for?

### Internal domain name

Every shoot cluster's kube-apiserver running in the seed is exposed via a load balancer that has a public endpoint (IP or hostname).
This endpoint is used by end-users and also by system components (that are running in another network, e.g., the kubelet or kube-proxy) to talk to the cluster.
In order to be robust against changes of this endpoint (e.g., caused due to re-creation of the load balancer or move of the DNS record to another seed cluster) Gardener creates a so-called *internal domain name* for every shoot cluster.
The *internal domain name* is a publicly resolvable DNS record that points to the load balancer of the kube-apiserver.
Gardener uses this domain name in the kubeconfigs of all system components, instead of using directly the load balancer endpoint.
This way Gardener does not need to recreate all kubeconfigs if the endpoint changes - it just needs to update the DNS record.

### External domain name

The internal domain name is not configurable by end-users directly but configured by the Gardener administrator.
However, end-users usually prefer to have another DNS name, maybe even using their own domain sometimes to access their Kubernetes clusters.
Gardener supports that by creating another DNS record, named *external domain name*, that actually points to the *internal domain name*.
The kubeconfig handed out to end-users does contain this *external domain name*, i.e., users can access their clusters with the DNS name they like to.

As not every end-user has an own domain it is possible for Gardener administrators to configure so-called *default domains*.
If configured, shoots that do not specify a domain explicitly get an *external domain name* based on a default domain (unless explicitly stated that this shoot should not get an external domain name (`.spec.dns.provider=unmanaged`).

### Ingress domain name (deprecated)

Gardener allows to deploy a `nginx-ingress-controller` into a shoot cluster (deprecated).
This controller is exposed via a public load balancer (again, either IP or hostname).
Gardener creates a wildcard DNS record pointing to this load balancer.
`Ingress` resources can later use this wildcard DNS record to expose underlying applications.

## What needs to be implemented to support a new DNS provider?

As part of the shoot flow Gardener will create a number of `DNSRecord` resources in the seed cluster (one for each of the DNS records mentioned above) that need to be reconciled by an extension controller.
This resource contains the following information:

* The DNS provider type (e.g., `aws-route53`, `google-clouddns`, ...)
* A reference to a `Secret` object that contains the provider-specific credentials used to communicate with the provider's API.
* The fully qualified domain name (FQDN) of the DNS record, e.g. "api.\<shoot domain\>".
* The DNS record type, one of `A`, `CNAME`, or `TXT`.
* The DNS record values, that is a list of IP addresses for A records, a single hostname for CNAME records, or a list of texts for TXT records.

Optionally, the `DNSRecord` resource may contain also the following information:

* The region of the DNS record. If not specified, the region specified in the referenced `Secret` shall be used. If that is also not specified, the extension controller shall use a certain default region. 
* The DNS hosted zone of the DNS record. If not specified, it shall be determined automatically by the extension controller by getting all hosted zones of the account and searching for the longest zone name that is a suffix of the fully qualified domain name (FQDN) mentioned above. 
* The TTL of the DNS record in seconds. If not specified, it shall be set by the extension controller to 120.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: dnsrecord-external
  namespace: shoot--foo--bar
type: Opaque
data:
  # aws-route53 specific credentials here
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: DNSRecord
metadata:
  name: dnsrecord-external
  namespace: default
spec:
  type: aws-route53
  secretRef:
    name: dnsrecord-external
    namespace: shoot--foo--bar
# region: eu-west-1
# zone: ZFOO
  name: api.bar.foo.my-fancy-domain.com
  recordType: A
  values:
  - 1.2.3.4
# ttl: 600
```

In order to support a new DNS record provider you need to write a controller that watches all `DNSRecord`s with `.spec.type=<my-provider-name>`.
You can take a look at the below referenced example implementation for the AWS route53 provider.

## Key names in secrets containing provider-specific credentials

For compatibility with existing setups, extension controllers shall support two different namings of keys in secrets containing provider-specific credentials:

* The naming used by the [external-dns-management DNS controller](https://github.com/gardener/external-dns-management). For example on AWS, the key names are `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION`.
* The naming used by other provider-specific extension controllers, e.g. for [infrastructure](infrastructure.md). For example on AWS, the key names are `accessKeyId`, `secretAccessKey`, and `region`.

## Avoiding reading the DNS hosted zones

If the DNS hosted zone is not specified in the `DNSRecord` resource, during the first reconciliation the extension controller shall determine the correct DNS hosted zone for the specified FQDN and write it to the status of the resource:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: DNSRecord
metadata:
  name: dnsrecord-external
  namespace: shoot--foo--bar
spec:
  ...
status:
  lastOperation: ...
  zone: ZFOO
```

On subsequent reconciliations, the extension controller shall use the zone from the status and avoid reading the DNS hosted zones from the provider.
If the `DNSRecord` resource specifies a zone in `.spec.zone` and the extension controller has written a value to `.status.zone`, the first one shall be considered with higher priority by the extension controller.

## Non-provider specific information required for DNS record creation

Some providers might require further information that is not provider specific but already part of the shoot resource.
As Gardener cannot know which information is required by providers it simply mirrors the `Shoot`, `Seed`, and `CloudProfile` resources into the seed.
They are part of the [`Cluster` extension resource](cluster.md) and can be used to extract information that is not part of the `DNSRecord` resource itself.

## References and additional resources

* [`DNSRecord` API (Golang specification)](../../pkg/apis/extensions/v1alpha1/types_dnsrecord.go)
* [Sample implementation for the AWS route53 provider](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller/dnsrecord)
