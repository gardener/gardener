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
- [GCP](https://github.com/gardener/gardener-extension-provider-gcp)
- [MetalStack](https://github.com/metal-stack/gardener-extension-provider-metal)
- [OpenStack](https://github.com/gardener/gardener-extension-provider-openstack)
- [Packet](https://github.com/gardener/gardener-extension-provider-packet)
- [vSphere](https://github.com/gardener/gardener-extension-provider-vsphere)
- [Kubevirt](https://github.com/gardener/gardener-extension-provider-kubevirt)

### DNS Provider

- [External DNS Management](https://github.com/gardener/external-dns-management) [*]

<sub>[*] Alicoud DNS, AWS Route53, Azure DNS, Cloudflare DNS, Google CloudDNS, OpenStack Designate</sub>

### Operating System

- [CoreOS/FlatCar](https://github.com/gardener/gardener-extension-os-coreos)
- [CoreOS/FlatCar (Alicloud)](https://github.com/gardener/gardener-extension-os-coreos-alicloud)
- [GardenLinux](https://github.com/gardener/gardener-extension-os-gardenlinux)
- [Debian/Ubuntu (MetalStack)](https://github.com/metal-stack/os-metal-extension)
- [Ubuntu](https://github.com/gardener/gardener-extension-os-ubuntu)
- [Ubuntu (Alicloud)](https://github.com/gardener/gardener-extension-os-ubuntu-alicloud)
- [SuSE CHost](https://github.com/gardener/gardener-extension-os-suse-chost)

### Container Runtime

- [gVisor](https://github.com/gardener/gardener-extension-runtime-gvisor)

### Network Plugin

- [Calico](https://github.com/gardener/gardener-extension-networking-calico)
- [Cilium](https://github.com/gardener/gardener-extension-networking-cilium)

### Generic Extensions

- [Shoot Certificate Service](https://github.com/gardener/gardener-extension-shoot-cert-service)
- [Shoot DNS Service](https://github.com/gardener/gardener-extension-shoot-dns-service)

If you implemented a new extension, please feel free to add it to this list!

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/gardener/gardener/issues) or join our [Slack channel #gardener](https://kubernetes.slack.com/messages/gardener) (please invite yourself to the Kubernetes workspace [here](http://slack.k8s.io)).

## Learn more!

Please find further resources about out project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)
* [GEP-1 (Gardener Enhancement Proposal) on extensibility](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md)
* [Extensibility API documentation](https://github.com/gardener/gardener/tree/master/docs/extensions)
