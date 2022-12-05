---
title: IPv6 Single-Stack Support in Local Gardener
gep-number: 21
creation-date: 2022-11-21
status: implementable
authors:
- "@einfachnuralex"
- "@timebertt"
reviewers:
- "TBD"
---

# GEP-21: IPv6 Single-Stack Support in Local Gardener

## Table of Contents

<!-- TOC -->
* [GEP-21: IPv6 Single-Stack Support in Local Gardener](#gep-21--ipv6-single-stack-support-in-local-gardener)
  * [Table of Contents](#table-of-contents)
  * [Summary](#summary)
  * [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
  * [Proposal](#proposal)
    * [`Shoot` API](#shoot-api)
    * [`Seed` API](#seed-api)
    * [`DNSRecord` API](#dnsrecord-api)
    * [Implementation Overview](#implementation-overview)
      * [Preparing the Local Setup](#preparing-the-local-setup)
      * [Scheduling](#scheduling)
      * [DNS Records](#dns-records)
      * [Network Policies](#network-policies)
      * [Shoot Worker Node Kernel Configuration](#shoot-worker-node-kernel-configuration)
      * [Docker Hub Images](#docker-hub-images)
      * [E2E Tests](#e2e-tests)
      * [Networking Extensions](#networking-extensions)
  * [Alternatives Considered](#alternatives-considered)
<!-- TOC -->

## Summary

Today, all shoot clusters provisioned by Gardener use IPv4 single-stack networking.
This GEP proposes changes to support IPv6 single-stack networking for shoots using provider-local.
Additionally, it considers future enhancements to support IPv4/IPv6 dual-stack networking for shoots without implementing it in the same step as IPv6 single-stack networking.

## Motivation

There is a need to cover additional scenarios to what Gardener currently provides and to configure different shoot networking setups.
Factors for changing the status quo include but are not limited to: the underlying cloud infrastructure, and the scarcity/cost of IPv4 addresses.
These different networking setups most prominently include IPv6 single-stack and IPv4/IPv6 dual-stack.
Kubernetes already supports such setups as described in [this doc](https://kubernetes.io/docs/concepts/services-networking/dual-stack/).
However, supporting these in Gardener requires changes to the API and many components (networking extensions, VPN tunnel, etc.).
For example, many components listen on `0.0.0.0` which makes them reachable via IPv4 only.

Supporting these kinds of networking setups in shoot clusters is a complex endeavour.
Hence, we propose to take several steps towards the ultimate goal of supporting all three of: IPv4 single-stack (already available today), IPv6 single-stack, IPv4/IPv6 dual-stack – both in the local setup and on cloud infrastructure.
As a first step, this GEP proposes support for IPv6 single-stack networking in the local Gardener environment only.
This keeps things focused on the most important changes to the API and central components while neglecting infrastructure-specific quirks and other difficulties that will arise with dual-stack networking.

While focusing on IPv6 single-stack networking for now, we cannot neglect how a future IPv4/IPv6 dual-stack implementation may look like and provide a corresponding outlook.
However, these ideas only serve an informational purpose for motivating decisions regarding the IPv6 single-stack implementation.
Once this enhancement has been implemented and IPv6 single-stack networking in local shoots is a stable feature, further changes can be proposed to support IPv6 single-stack on cloud infrastructure, and eventually IPv4/IPv6 dual-stack networking.

### Goals

- augment all relevant Gardener API types defined in gardener/gardener to allow selecting either IPv4 or IPv6 single-stack networking (`core.gardener.cloud/v1beta1.{Shoot,Seed}`, `extensions.gardener.cloud/v1alpha1.{Network,DNSRecord}`)
- define a contract that all components (including networking extensions) need to follow to support this configuration
- adapt all relevant components in the local Gardener environment to support this configuration
- document all required steps for getting started with the IPv6 setup for developers (without requiring existing config/knowledge)
- add e2e tests to prevent regressions and guarantee the stability of the feature (especially because nobody is running it in productive environments)

### Non-Goals

- support IPv4/IPv6 dual-stack networking
- support IPv6 single-stack networking on cloud infrastructure
- support changing the networking setup of shoots from IPv4 to IPv6 single-stack or vice-versa
- propose changes API types defined outside of gardener/gardener and their implementations, e.g., the `NetworkConfig` APIs (`providerConfig`) of networking extensions
- support the legacy VPN tunnel (`ReversedVPN` feature gate is disabled)

## Proposal

A new feature gate `IPv6SingleStack` is added to gardener-apiserver.
The IPv6-related fields and values can only be used if the feature gate is enabled.
The feature gate cannot be enabled if the `ReversedVPN` feature gate is disabled (the legacy VPN tunnel solution is [not supported by provider-local](https://github.com/gardener/gardener/blob/83de074f1bc1c009f92e97a08289340591377af6/docs/extensions/provider-local.md#current-limitations) anyway).

The feature gate serves the purpose of disabling the feature in productive Gardener installations and prevents users from configuring IPv6 networking for their shoot clusters, while it is still under development and not supported on cloud infrastructure.
As part of this enhancement, the feature gate is supposed to be enabled only in the local environment.
Later on, the feature gate may also be used for safe-guarding maturing and enablement of a IPv6 single-stack implementation on cloud infrastructure.
The feature gate is not supposed to be toggled back and forth.

### `Shoot` API

The `Shoot.spec.networking` section is extended as follows:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  networking:
    # ...
    ipFamilies:
    - IPv6
    podsV6: 2001:db8:1::/48
    nodesV6: 2001:db8:2::/48
    servicesV6: 2001:db8:3::/48
```

`ipFamilies` is the central setting for specifying the IP families used for shoot networking.
This field is inspired by the `Service.spec.ipFamilies` field in Kubernetes ([doc](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services)).
The default value is `["IPv4"]` (IPv4 single-stack).
If the `IPv6SingleStack` feature gate is enabled, `["IPv6"]` can be specified to switch to IPv6 single-stack.

Later on, `["IPv4","IPv6"]` or `["IPv6","IPv4"]` can be supported for dual-stack networking.
In dual-stack networking, the ordering of `ipFamilies` is needed for correctly configuring Kubernetes components, e.g., the API server's `--service-cluster-ip-range` flag controls which IP family is used to allocate the primary `clusterIP` of services.

Instead of explicitly configuring the `ipFamilies` field, the used families could be determined implicitly from the corresponding CIDR fields.
However, when using IPv6 single-stack networking on cloud infrastructure, users might want to use IPv6 prefixes assigned to them by the provider.
Typically, these are not known by the user upfront and need to be allocated during cluster creation.
In this case, users might not supply any of the CIDRs in their shoot specification.
Hence, we need a central field that allows configuring the used IP families explicitly instead of implicitly.
That's exactly the purpose of the `ipFamilies` field.

Additionally, IPv6-equivalents of all CIDR and mask fields are introduced, e.g., `Shoot.spec.networking.{pods,services,nodes}V6` and `Shoot.spec.kubernetes.kubeControllerManager.nodeCIDRMaskSizeV6`.
Depending on the `ipFamilies` value, either the IPv4 field or the IPv6 field may be specified – or both in a potential dual-stack implementation.

All gardener components and extensions need to respect the `ipFamilies` field and handle it correctly, e.g., in API validation, defaulting, and configuring Shoot components.

### `Seed` API

Similar to the `Shoot` API, the `Seed.spec.networks` section is extended as follows:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
spec:
  networks:
    # ...
    ipFamilies:
    - IPv6
    nodesV6: 2001:db8:11::/48
    podsV6: 2001:db8:12::/48
    servicesV6: 2001:db8:13::/48
    shootDefaults:
      # ...
      podsV6: 2001:db8:1::/48
      servicesV6: 2001:db8:3::/48
```

`ipFamilies` has the same semantics as `Shoot.spec.networking.ipFamilies`.
Again, IPv6-equivalents of all CIDR and mask fields are introduced, e.g. `Seed.spec.networks.{nodes,pods,services}V6` and `Seed.spec.networks.shootDefaults.{pods,services}V6`
Depending on the `ipFamilies` value, either the IPv4 field or the IPv6 field may be specified – or both in a potential dual-stack implementation.

The existing `Seed.spec.networks.blockCIDRs` field is augmented to allow IPv6 CIDR values in addition to IPv4 values.

### `Network` API

The `Network` API is augmented analogously to the `Shoot.spec.networking` section:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
spec:
  # ...
  ipFamilies:
  - IPv6
  podCIDRv6: 2001:db8:1::/48
  serviceCIDRv6: 2001:db8:3::/48
```

Similar to the `Shoot` API, a new `ipFamilies` field is introduced along with IPv6-equivalents of all CIDR fields, i.e.., `Network.spec.{pod,service}CIDRv6`.
As with the existing `Network` fields, these fields are filled by gardenlet according to the `Shoot` specification on reconciliations.

### `DNSRecord` API

The `DNSRecord` API validation is changed to allow creating `AAAA` records:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: DNSRecord
spec:
  # ...
  recordType: AAAA
  values:
  - 2001:db8:f00::1
```

- `spec.recordType` allows specifying `AAAA` in addition to the current set of valid record types
- `spec.values` allows specifying IPv6 values if `spec.recordType=AAAA`

### Implementation Overview

This section gives a rough overview of the most important changes for handling the `ipFamilies` fields.
We won't go into detail here but only highlight a few selected changes.
The most important takeaway is, that all components need to respect the `ipFamilies` fields.

#### Preparing the Local Setup

The kind cluster (used as garden runtime and seed cluster) is switched to IPv6 single-stack networking.
This includes the kind cluster configuration itself as well as its calico configuration.
Developers will need to perform some configuration of their machine and Docker installation.
Corresponding instructions are made available for macOS and Linux machines.

provider-local is configured to use an IPv6 address (`::1`) instead of `127.0.0.1` for patching the status of LoadBalancer services.

#### Scheduling

gardener-apiserver only allows scheduling shoots to seeds with matching `ipFamilies` settings.
I.e., all `Shoot.spec.networking.ipFamilies` values must be included in `Seed.spec.networks.ipFamilies`.
This might be changed later on if "networking magic" is in place to host dual-stack shoots on IPv6 singe-stack seeds or similar.

Also, gardener-scheduler chooses seeds with `ipFamilies` settings matching the shoot's configuration.

#### DNS Records

gardenlet creates `DNSRecord` objects for the shoot API server with the record type corresponding to the `Shoot.spec.networking.ipFamilies` setting (`A` for IPv4, `AAAA` for IPv6).
`AAAA` records are created with a suffix to avoid name collisions in a potential dual-stack implementation.

#### Network Policies

Gardener manages several `NetworkPolicy` objects with static CIDRs (e.g. `allow-to-public-networks` in seeds).
For these, IPv6-equivalents of the IPv4 CIDRs are added, e.g., `::/0` as an equivalent to `0.0.0.0/0`.

#### Shoot Worker Node Kernel Configuration

Kubernetes networking requires IPv6 forwarding to be enabled on the OS level.
Hence, gardenlet explicitly enables the corresponding kernel setting for shoot worker nodes via `OperatingSystemConfigurations`, similar to IPv4 (ref [gardener/gardener#7046](https://github.com/gardener/gardener/pull/7046)).

#### Docker Hub Images

The `docker.io` registry doesn't support pulling images over IPv6 (see [Beta IPv6 Support on Docker Hub Registry](https://www.docker.com/blog/beta-ipv6-support-on-docker-hub-registry/)).

Container images from `docker.io` used on shoots and seeds are rewritten to `registry.ipv6.docker.com` if the corresponding `ipFamilies` field specifies IPv6 singe-stack.

#### E2E Tests

Small changes in the OS and networking stack might lead to regressions of the IPv6 feature.
Hence, [e2e tests](../development/testing.md#end-to-end-e2e-tests-using-provider-local) (presubmits and periodics) are added to prevent unnoticed regressions of the feature.

#### Networking Extensions

Networking extensions need to support the configuration of IPv6-related settings of the networking implementation in their `NetworkConfig` API.
This is not specified any further by gardener and will be implemented differently in different extensions (as the `NetworkConfig` API design differs already today).
In general, extensions need to respect the `Shoot.spec.networking.ipFamilies` settings in all aspects: API validation, defaulting, and corresponding handling in code (i.e., configuring IPAM, etc.).

## Alternatives Considered

Instead of adding IPv6-equivalent fields in the `Shoot` and `Seed` API (e.g., `Shoot.spec.networking.{pods,services,nodes}V6`), new arrays could be added that allow specifying both settings in a single list (similar to `Service.spec.clusterIPs`).
However, to guarantee backward and forward compatibility, the new array would need to be kept in sync with the existing field and the existing field must not be removed.
This makes API validation more complex and might confuse users.
For the sake of simplicity and a more expressive and less confusing API, this approach was discarded.
However, a potential `core.gardener.cloud/v1` API might choose to use a single array for both settings instead of separate fields.
