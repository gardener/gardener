---
title: 21 IPv6 Single-Stack Support in Local Gardener
gep-number: 21
creation-date: 2022-11-21
status: implementable
authors:
- "@einfachnuralex"
- "@timebertt"
reviewers:
- "@ScheererJ"
- "@rfranzke"
---

# GEP-21: IPv6 Single-Stack Support in Local Gardener

## Table of Contents

<!-- TOC -->
- [GEP-21: IPv6 Single-Stack Support in Local Gardener](#gep-21-ipv6-single-stack-support-in-local-gardener)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [`Shoot` API](#shoot-api)
      - [Future Dual-Stack Enhancements](#future-dual-stack-enhancements)
    - [`Seed` API](#seed-api)
    - [`Network` API](#network-api)
    - [`DNSRecord` API](#dnsrecord-api)
    - [Implementation Overview](#implementation-overview)
      - [Preparing the Local Setup](#preparing-the-local-setup)
      - [DNS Records](#dns-records)
      - [Network Policies](#network-policies)
      - [Shoot Worker Node Kernel Configuration](#shoot-worker-node-kernel-configuration)
      - [Docker Hub Images](#docker-hub-images)
      - [E2E Tests](#e2e-tests)
      - [Networking Extensions](#networking-extensions)
  - [Alternatives Considered](#alternatives-considered)
<!-- TOC -->

## Summary

Today, all shoot clusters provisioned by Gardener use IPv4 single-stack networking.
This GEP proposes changes to support IPv6 single-stack networking for shoots using provider-local.
Additionally, it considers future enhancements to support IPv4/IPv6 dual-stack networking for shoots without implementing it in the same step as IPv6 single-stack networking.

## Motivation

There is a need to cover additional scenarios to what Gardener currently provides and to configure different shoot networking setups.
Factors for changing the status quo include but are not limited to: the underlying cloud infrastructure and the scarcity/cost of IPv4 addresses.
These different networking setups most prominently include IPv6 single-stack and IPv4/IPv6 dual-stack.
Kubernetes already supports such setups as described in the [IPv4/IPv6 dual-stack](https://kubernetes.io/docs/concepts/services-networking/dual-stack/) topic.
However, supporting these in Gardener requires changes to the API and many components (networking extensions, VPN tunnel, etc.).
For example, many components listen on `0.0.0.0`, which makes them reachable via IPv4 only.

Supporting these kinds of networking setups in shoot clusters is a complex endeavour.
Hence, we propose to take several steps towards the ultimate goal of supporting all three of: IPv4 single-stack (already available today), IPv6 single-stack, and IPv4/IPv6 dual-stack – both in the local setup and on cloud infrastructure.
As a first step, this GEP proposes support for IPv6 single-stack networking in the local Gardener environment only.
This keeps things focused on the most important changes to the API and central components, while neglecting infrastructure-specific quirks and other difficulties that will arise with dual-stack networking.
For example, when using IPv6 single-stack networking on cloud infrastructure, users might want to use IPv6 prefixes assigned to them by the provider instead of configuring the shoot's node and pod CIDRs upfront.

While focusing on IPv6 single-stack networking for now, we cannot neglect how a future IPv4/IPv6 dual-stack implementation may look like and provide a corresponding outlook.
However, these ideas only serve an informational purpose for motivating decisions regarding the IPv6 single-stack implementation.
Once this enhancement has been implemented and IPv6 single-stack networking in local shoots is a stable feature, further changes can be proposed to support IPv6 single-stack on cloud infrastructure, and eventually IPv4/IPv6 dual-stack networking.

### Goals

- Augment all relevant Gardener API types defined in gardener/gardener to allow selecting either IPv4 or IPv6 single-stack networking (`core.gardener.cloud/v1beta1.{Shoot,Seed}`, `extensions.gardener.cloud/v1alpha1.{Network,DNSRecord}`).
- Define a contract that all components (including networking extensions) need to follow to support this configuration.
- Adapt all relevant components in the local Gardener environment to support this configuration.
- Document all required steps for getting started with the IPv6 setup for developers (without requiring existing config/knowledge).
- Add e2e tests to prevent regressions and guarantee the stability of the feature (especially because nobody is running it in productive environments).

### Non-Goals

- Support IPv4/IPv6 dual-stack networking.
- Provide IPv4 connectivity on IPv6 single-stack clusters.
- Support IPv6 single-stack networking on cloud infrastructure.
- Use IPv6 prefixes assigned by provider for pod CIDR similar to [provider-assigned node CIDR](https://github.com/gardener/gardener/blob/219d828fcdea81fb3edf13de2736daf81e137923/pkg/operation/botanist/infrastructure.go#L73).
- Support changing the networking setup of shoots from IPv4 to IPv6 single-stack or vice-versa.
- Host IPv6 single-stack shoots on IPv4 single-stack seeds or vice-versa.
- Propose changes to API types defined outside of gardener/gardener and their implementations, e.g., the `NetworkConfig` APIs (`providerConfig`) of networking extensions.

## Proposal

A new feature gate `IPv6SingleStack` is added to gardener-apiserver.
The IPv6-related fields and values can only be used if the feature gate is enabled.

The feature gate serves the purpose of disabling the feature in productive Gardener installations and prevents users from configuring IPv6 networking for their shoot clusters while it is still under development and not supported on cloud infrastructure.
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
    pods: 2001:db8:1::/48
    nodes: 2001:db8:2::/48
    services: 2001:db8:3::/48
    # ...
    ipFamilies:
    - IPv6
```

`ipFamilies` is the central setting for specifying the IP families used for shoot networking.
This field is inspired by the `Service.spec.ipFamilies` field in Kubernetes (see [Services](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services)).
The field is defaulted to `["IPv4"]` (IPv4 single-stack) if not set.
With this, this field is set to its implicit value for all existing shoots to provide backward-compatibility.
If the `IPv6SingleStack` feature gate is enabled, `["IPv6"]` can be specified to switch to IPv6 single-stack.

Later on, `["IPv4","IPv6"]` or `["IPv6","IPv4"]` can be supported for dual-stack networking.
In dual-stack networking, the ordering of `ipFamilies` is needed for correctly configuring Kubernetes components, e.g., the API server's `--service-cluster-ip-range` flag controls which IP family is used to allocate the primary `clusterIP` of services.

Instead of explicitly configuring the `ipFamilies` field, the used families could be determined implicitly from the corresponding CIDR fields.
However, when using IPv6 single-stack networking on cloud infrastructure, users might want to use IPv6 prefixes assigned to them by the provider.
Typically, these are not known by the user upfront and need to be allocated during cluster creation.
In this case, users might not supply any of the CIDRs in their shoot specification.
Hence, we need a central field that allows configuring the used IP families explicitly instead of implicitly.
That's exactly the purpose of the `ipFamilies` field.

If IPv6 single-stack networking is configured via the `ipFamilies` field, the existing `Shoot.spec.networking.{pods,services,nodes}` fields are used to specify the IPv6 CIDRs instead of the IPv4 CIDRs.

All Gardener components and extensions need to respect the `ipFamilies` field and handle it correctly, e.g., in API validation, defaulting, and configuring Shoot components.

#### Future Dual-Stack Enhancements

For supporting dual-stack networking setups in the future, the `Shoot` API must allow specifying CIDRs of both IP families.
Similarly to the [`Service` API](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services), the `Shoot` API may be extended with list equivalents of `Shoot.spec.networking.{pods,services,nodes}`.
In this case, the existing CIDR fields may specify the respective CIDR of the primary IP family while the list fields contain CIDRs of both IP families.
The primary IP family may be determined by the first element of the `ipFamilies` field.
Similarly to the `Service.spec.clusterIPs` field, API validation may enforce that the list only contains two entries with the primary IP family being the first one.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  networking:
    pods: 2001:db8:1::/48
    nodes: 2001:db8:2::/48
    services: 2001:db8:3::/48
    ipFamilies:
    - IPv6
    - IPv4
    # ...
    podsCIDRs:
    - 2001:db8:1::/48
    - 10.0.1.0/24
    nodesCIDRs:
    - 2001:db8:2::/48
    - 10.0.2.0/24
    servicesCIDRs:
    - 2001:db8:3::/48
    - 10.0.3.0/24
```

The new and existing CIDR API fields are immutable, similar to the `Service.spec.clusterIP` field ([ref](https://github.com/kubernetes/kubernetes/blob/release-1.24/pkg/apis/core/validation/validation.go#L4828-L4831)).
Hence, there won't be any complex logic for syncing the primary CIDR to the list and vice-versa.

Corresponding changes may be performed to the other relevant APIs, e.g., the `Seed` and `Network` APIs.

### `Seed` API

Similarly to the `Shoot` API, the `Seed.spec.networks` section is extended as follows:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
spec:
  networks:
    nodes: 2001:db8:11::/48
    pods: 2001:db8:12::/48
    services: 2001:db8:13::/48
    shootDefaults:
      pods: 2001:db8:1::/48
      services: 2001:db8:3::/48
    # ...
    ipFamilies:
    - IPv6
```

`ipFamilies` has the same semantics as `Shoot.spec.networking.ipFamilies`.
Again, the existing CIDR fields are used to specify the IPv6 CIDRs instead of IPv4 CIDRs, e.g. `Seed.spec.networks.{nodes,pods,services}` and `Seed.spec.networks.shootDefaults.{pods,services}`.
Similarly to the other networking-related fields in the `Seed` API, the `ipFamilies` field has an informational character only.
It can be used to handle IP family-specifics (see [Docker Hub Images](#docker-hub-images)) and may be used for restricting the scheduling of shoots to seeds with matching IP families.

The existing `Seed.spec.networks.blockCIDRs` field is augmented to allow IPv6 CIDR values in addition to IPv4 values.

### `Network` API

The `Network` API is augmented analogously to the `Shoot.spec.networking` section:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
spec:
  podCIDR: 2001:db8:1::/48
  serviceCIDR: 2001:db8:3::/48
  # ...
  ipFamilies:
  - IPv6
```

Similarly to the `Shoot` API, a new `ipFamilies` field is introduced.
Again, the existing CIDR fields are used to specify the IPv6 CIDRs instead of IPv4 CIDRs, e.g., `Network.spec.{pod,service}CIDR`.

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

- `spec.recordType` allows specifying `AAAA` in addition to the current set of valid record types.
- `spec.values` allows specifying IPv6 values if `spec.recordType=AAAA`.

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

#### DNS Records

The gardenlet creates `DNSRecord` objects for the shoot API server with the record type corresponding to the `Shoot.spec.networking.ipFamilies` setting (`A` for IPv4, `AAAA` for IPv6).
`AAAA` records are created with a suffix to avoid name collisions in a potential dual-stack implementation.

#### Network Policies

Gardener manages several `NetworkPolicy` objects with static CIDRs (e.g. `allow-to-public-networks` in seeds).
For these, IPv6-equivalents of the IPv4 CIDRs are added, e.g., `::/0` as an equivalent to `0.0.0.0/0`.

#### Shoot Worker Node Kernel Configuration

Kubernetes networking requires IPv6 forwarding to be enabled on the OS level.
Hence, the gardenlet explicitly enables the corresponding kernel setting for shoot worker nodes via `OperatingSystemConfigurations`, similar to IPv4 (ref [gardener/gardener#7046](https://github.com/gardener/gardener/pull/7046)).

#### Docker Hub Images

The `docker.io` registry doesn't support pulling images over IPv6 (see [Beta IPv6 Support on Docker Hub Registry](https://www.docker.com/blog/beta-ipv6-support-on-docker-hub-registry/)).

Container images from `docker.io` used on shoots and seeds are rewritten to `registry.ipv6.docker.com` if the corresponding `ipFamilies` field specifies IPv6 singe-stack.

#### E2E Tests

Small changes in the OS and networking stack might lead to regressions of the IPv6 feature.
Hence, [e2e tests](../development/testing.md#end-to-end-e2e-tests-using-provider-local) (presubmits and periodics) are added to prevent unnoticed regressions of the feature.

#### Networking Extensions

Networking extensions need to support the configuration of IPv6-related settings of the networking implementation in their `NetworkConfig` API.
The concrete API changes and implementation thereof are not specified any further by this GEP.
It is expected that IPv6 support will be implemented differently in different extensions, similar to differences in the `NetworkConfig` API design that are already present today.
In general, extensions need to respect the `Shoot.spec.networking.ipFamilies` settings in all aspects: API validation, defaulting, and corresponding handling in code (i.e., configuring IPAM, etc.).

As this GEP aims for creating IPv6 single-stack shoots in the local environment, it depends on the IPv6 implementation in networking-calico, which is currently used as the networking extension for local shoots.
Hence, we only list a few important steps of the implementation here without going into detail:

- The `NetworkConfig` API is extended to allow specifying IPv6-related settings similar to IPv4-related settings (pool, auto-detection, etc.).
- IPv4 support is disabled on calico-node via environment variables.
- IPv4 IPAM is disabled on calico-node via configuration.
- IPv6 support is enabled on calico-node via environment variables (IPv6 auto-detection, IPv6 pod CIDR, etc.).
- IPv6 IPAM is enabled on calico-node via configuration.

## Alternatives Considered

Instead of reusing the existing CIDR fields in the `Shoot`, `Seed`, and `Network` APIs, new fields could be added that allow specifying the IPv6 CIDRs only, e.g., `Shoot.spec.networking.{pods,services,nodes}V6`.
This could simplify handling in code (e.g., validation and defaulting logic) in a future dual-stack implementation.
We decided to reuse the existing CIDR fields for specifying the IPv6 CIDRs in a single-stack setup and introduce additional lists for specifying the secondary CIDRs in a dual-stack setup, as it resembles the `Service` API, is cleaner from an API design perspective, and is less confusing for shoot owners.
