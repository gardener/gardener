---
title: Rate limits for DNS entries created by shoot-dns-service
gep-number: 17
creation-date: 2021-05-03
status: implementable
authors:
- "@MartinWeindel"
reviewers:
- "@timuthy"
---

# GEP-17: Rate limits for DNS entries created by shoot-dns-service

## Table of Contents

- [GEP-17: Rate limits for DNS entries created by shoot-dns-service](#gep-17-rate-limits-for-dns-entries-created-by-shoot-dns-service)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [DNS controller manager](#dns-controller-manager)
    - [Gardener](#gardener)
  - [Alternatives](#alternatives)

## Summary

This `GEP` introduces rate limits for `DNSEntries` created with the help of the [shoot-dns-service](https://github.com/gardener/gardener-extension-shoot-dns-service) to protect the [dns-controller-manager](https://github.com/gardener/external-dns-management) from being throttled, i.e to protect the DNS management of Gardener from intentional or unintentional *DoS* attacks.

## Motivation

The `dns-controller-manager` runs update requests for DNS record on the configured DNS cloud service(s) like `AWS Route 53`. Typically these DNS cloud services are rate limiting requests on account level. As the `shoot-dns-service` allows to request and update `DNSEntries` directly or indirectly from resources on the shoot cluster, a single shoot cluster can potentially eat up the available request rate of the backend account. The `dns-controller-manager` runs as extension on each seed of a Gardener landscape and they all have to share the account for the DNS cloud service in form of a `DNSProvider` as all shoots use the same default *hosted DNS zone* for `DNSEntries` created by Gardener itself ( `external`, `ingress`) and all others created by the `shoot-dns-service`. It is this default `DNSProvider` which needs to be protected at first place. If the user provides an own `DNSProvider` and uses his hosted DNS zone(s) of his own account, the throttling would only affect this account. But also in this case, the user may want to avoid that one of his shoot clusters can break all his others clusters using the same `DNSProvider`.

### Goals

- The rate limit is set and applied separatedly for each `DNSProvider`
- All update requests caused by `DNSEntries` created or updated from the `shoot-dns-service` should only be forwarded to the backend DNS service, if the update requests are within rate limit of the `DNSProvider` the `DNSEntry` is assigned to.
- Deletion of `DNSEntries` is not rate limited to avoid freezing on shoot cluster deletion.
- Updates of `DNSEntries` created by Gardener (e.g. `internal`, `external`, and `ingress`) should not be blocked by the rate limits.
- If creation/updates of `DNSEntries` of the `shoot-dns-service` are pending because of a rate limit, this should also made be visible in the shoot cluster (either by events or status update)

### Non-Goals

- prioritize updates between `DNSEntries` assigned to the same `DNSProvider`

## Proposal

This proposal involves changes in two components:

- [dns-controller-manager](https://github.com/gardener/external-dns-management) for implementation of rate limiting

- [gardener](https://github.com/gardener/gardener) for configuration of rate limits, applying them to the external `DNSProvider` and annotating `DNSEntries` to exclude then from rate limiting

### DNS controller manager

The spec of a `DNSProvider` is extended by an optional `rateLimit` section.

```go
type DNSProviderSpec struct {
    ...
    // rate limit for create/update operations on DNSEntries assigned to this provider
    // +optional
    RateLimit *RateLimit `json:"rateLimit,omitempty"`
}

type RateLimit struct {
    // RequestsPerDay is create/update request rate per DNS entry given by requests per day
    RequestsPerDay int `json:"requestsPerDay"`
    // Burst allows bursts of up to 'burst' to exceed the rate defined by 'RequestsPerDay', while still maintaining a
    // smoothed rate of 'RequestsPerDay'
    Burst int `json:"burst"`
}
```

Example manifest:

```yaml
kind: DNSProvider
...
spec:
  ...
  rateLimit:
    requestsPerDay: 120
    burst: 20
```

Each valid `DNSEntry` is already automatically assigned to an `DNSProvider` by matching the domain name and the realm.
This assignment now also includes the regime of the rate limit of this `DNSProvider`.
The creation or update of DNS records is postponed until the rate limit restrictions are fulfilled.
The entry stays in the state `Pending` or `Updating` until the request is applied on the DNS cloud service.
Here the state `Updating` is a new, additional state to be implemented to make transparent that spec and backend are not yet synchronized.

Deletion of `DNSEntry` is always applied immediately to avoid possible extreme delays on shoot deletion.

As the status of `DNSEntries` created by the `shoot-dns-service` in the CP is already forwarded to the source objects in the shoot cluster in form
of events, nothing needs to be added here. Additionally, the status for shoot-side `DNSEntries` is updated accordingly.

On replicating a `DNSEntry` from the shoot cluster nothing must be changed, as annotations are not copied. This means a user cannot escape the rate limiting by adding an annotation like `dns.gardener.cloud/not-rate-limited: "true"`

### Gardener

The rate limit for the default external `DNSProvider` is configured for the default domains of the garden.
It must also be possible to set a rate-limit for an additional `DNSProvider` specified in the shoot manifest optionally.

On creating a new shoot (with enabled DNS), the external `DNSProvider` is created with the rate limit for the default domain or as specified in the shoot manifest for the primary `DNSProvider`.

The `DNSEntries` created by the gardenlet (`internal`, `external`, and `ingress`) should be annotated with `dns.gardener.cloud/not-rate-limited: "true"`.

## Alternatives

- In principle the `shoot-dns-service` could defer the creation/update of the `DNSEntries` in the control plane depending on the rate limit. But the problem is here, that this service has currently no logic to calculate the assignment to the correct `DNSProvider`. This logic is only available to the `dns-controller-manager` as it needs to watch all providers. Every `shoot-dns-service` would need to watch the `DNSProviders` itself and apply the assignment logic which is quite complex.
