# [Gardener Extensions Library](https://gardener.cloud)

![Gardener Extensions Logo](../logo/gardener-extensions-large.png)

Project Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service. Its main principle is to leverage Kubernetes concepts for all of its tasks.

Recently, most of the vendor specific logic has been developed [in-tree](https://github.com/gardener/gardener). However, the project has grown to a size where it is very hard to extend, maintain, and test. With [GEP-1](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md) we have proposed how the architecture can be changed in a way to support external controllers that contain their very own vendor specifics. This way, we can keep Gardener core clean and independent.

This directory contains utilities functions and common libraries meant to ease writing the actual extension controllers.
Please consult https://github.com/gardener/gardener/tree/master/docs/extensions to get more information about the extension contracts.

## Known Extension Implementations

Check out these repositories for implementations of the Gardener Extension contracts:

### Infrastructure Provider

- [Alicloud](https://github.com/gardener/gardener-extension-provider-alicloud)
- [AWS](https://github.com/gardener/gardener-extension-provider-aws)
- [Azure](https://github.com/gardener/gardener-extension-provider-azure)
- [Equinix Metal](https://github.com/gardener/gardener-extension-provider-equinix-metal)
- [GCP](https://github.com/gardener/gardener-extension-provider-gcp)
- [Hetzner Cloud](https://github.com/23technologies/gardener-extension-provider-hcloud)
- [Kubevirt](https://github.com/gardener/gardener-extension-provider-kubevirt)
- [MetalStack](https://github.com/metal-stack/gardener-extension-provider-metal)
- [OpenStack](https://github.com/gardener/gardener-extension-provider-openstack)
- [vSphere](https://github.com/gardener/gardener-extension-provider-vsphere)

### Primary DNS Provider

The primary DNS provider manages `DNSRecord` resources (mandatory for Gardener related DNS records)

- [Alicloud](https://github.com/gardener/gardener-extension-provider-alicloud)
- [AWS](https://github.com/gardener/gardener-extension-provider-aws)
- [Azure](https://github.com/gardener/gardener-extension-provider-azure)
- [CloudFlare](https://github.com/schrodit/gardener-extension-provider-dns-cloudflare)
- [GCP](https://github.com/gardener/gardener-extension-provider-gcp)
- [OpenStack](https://github.com/gardener/gardener-extension-provider-openstack)
- [PowerDNS](https://github.com/metal-stack/gardener-extension-dns-powerdns)
- [RFC2136](https://github.com/Avarei/gardener-extension-dns-rfc2136)

### Operating System

- [CoreOS/FlatCar](https://github.com/gardener/gardener-extension-os-coreos)
- [Debian/Ubuntu (MetalStack)](https://github.com/metal-stack/os-metal-extension)
- [GardenLinux](https://github.com/gardener/gardener-extension-os-gardenlinux)
- [k3os](https://github.com/23technologies/gardener-extension-os-k3os)
- [SuSE CHost](https://github.com/gardener/gardener-extension-os-suse-chost)
- [Ubuntu](https://github.com/gardener/gardener-extension-os-ubuntu)

### Container Runtime

- [gVisor](https://github.com/gardener/gardener-extension-runtime-gvisor)
- [Kata Containers](https://github.com/23technologies/gardener-extension-runtime-kata)

### Network Plugin

- [Calico](https://github.com/gardener/gardener-extension-networking-calico)
- [Cilium](https://github.com/gardener/gardener-extension-networking-cilium)

### Generic Extensions

- [Minimal Working Example](https://github.com/23technologies/gardener-extension-mwe)
- [Registry Cache](https://github.com/gardener/gardener-extension-registry-cache)
- [Shoot Certificate Service](https://github.com/gardener/gardener-extension-shoot-cert-service)
- [Shoot DNS Service](https://github.com/gardener/gardener-extension-shoot-dns-service)
- [Shoot Falco Service](https://github.com/gardener/gardener-extension-shoot-falco-service)
- [Shoot Flux Service](https://github.com/23technologies/gardener-extension-shoot-flux)
- [Shoot Lakom Service](https://github.com/gardener/gardener-extension-shoot-lakom-service)
- [Shoot Networking Filter](https://github.com/gardener/gardener-extension-shoot-networking-filter)
- [Shoot Networking Problem Detector](https://github.com/gardener/gardener-extension-shoot-networking-problemdetector)
- [Shoot OpenID Connect Service](https://github.com/gardener/gardener-extension-shoot-oidc-service)
- [Shoot Rsyslog Relp](https://github.com/gardener/gardener-extension-shoot-rsyslog-relp)

If you implemented a new extension, please feel free to add it to this list!

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/gardener/gardener/issues) or reach out on [Slack](https://gardener-cloud.slack.com/) (join the workspace [here](https://gardener.cloud/community/community-bio/)).

## Learn more!

Please find further resources about out project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)
* [GEP-1 (Gardener Enhancement Proposal) on extensibility](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md)
* [Extensibility API documentation](https://github.com/gardener/gardener/tree/master/docs/extensions)
