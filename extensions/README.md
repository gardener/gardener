# [Gardener Extensions Library](https://gardener.cloud)

![Gardener Extensions Logo](../logo/gardener-extensions-large.png)

Project Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service. Its main principle is to leverage Kubernetes concepts for all of its tasks.

Recently, most of the vendor specific logic has been developed [in-tree](https://github.com/gardener/gardener). However, the project has grown to a size where it is very hard to extend, maintain, and test. With [GEP-0001](https://github.com/gardener/enhancements/tree/main/geps/0001-gardener-extensibility) we have proposed how the architecture can be changed in a way to support external controllers that contain their very own vendor specifics. This way, we can keep Gardener core clean and independent.

This directory contains utilities functions and common libraries meant to ease writing the actual extension controllers.
Please consult https://github.com/gardener/gardener/tree/master/docs/extensions to get more information about the extension contracts.

## Known Extension Implementations

Check out these repositories for implementations of the Gardener Extension contracts:

### Infrastructure Provider

- [Alibaba Cloud](https://github.com/gardener/gardener-extension-provider-alicloud)
- [AWS](https://github.com/gardener/gardener-extension-provider-aws)
- [Azure](https://github.com/gardener/gardener-extension-provider-azure)
- [Equinix Metal](https://github.com/gardener/gardener-extension-provider-equinix-metal)
- [GCP](https://github.com/gardener/gardener-extension-provider-gcp)
- [Hetzner Cloud](https://github.com/23technologies/gardener-extension-provider-hcloud)
- [IronCore](https://github.com/ironcore-dev/gardener-extension-provider-ironcore)
- [IronCore Metal](https://github.com/ironcore-dev/gardener-extension-provider-ironcore-metal)
- [KubeVirt](https://github.com/gardener/gardener-extension-provider-kubevirt)
- [metal-stack](https://github.com/metal-stack/gardener-extension-provider-metal)
- [OpenStack](https://github.com/gardener/gardener-extension-provider-openstack)
- [STACKIT](https://github.com/stackitcloud/gardener-extension-provider-stackit)
- [vSphere](https://github.com/gardener/gardener-extension-provider-vsphere)

### Primary DNS Provider

The primary DNS provider manages `DNSRecord` resources (mandatory for Gardener related DNS records)

- [Alibaba Cloud](https://github.com/gardener/gardener-extension-provider-alicloud)
- [AWS](https://github.com/gardener/gardener-extension-provider-aws)
- [Azure](https://github.com/gardener/gardener-extension-provider-azure)
- [Cloudflare](https://github.com/schrodit/gardener-extension-provider-dns-cloudflare)
- [GCP](https://github.com/gardener/gardener-extension-provider-gcp)
- [OpenStack](https://github.com/gardener/gardener-extension-provider-openstack)
- [PowerDNS](https://github.com/metal-stack/gardener-extension-dns-powerdns)
- [RFC2136](https://github.com/Avarei/gardener-extension-dns-rfc2136)

### Operating System

- [CoreOS/Flatcar](https://github.com/gardener/gardener-extension-os-coreos)
- [Debian/Ubuntu (metal-stack)](https://github.com/metal-stack/os-metal-extension)
- [Garden Linux](https://github.com/gardener/gardener-extension-os-gardenlinux)
- [k3os](https://github.com/23technologies/gardener-extension-os-k3os)
- [SUSE CHost](https://github.com/gardener/gardener-extension-os-suse-chost)
- [Ubuntu](https://github.com/gardener/gardener-extension-os-ubuntu)

### Container Runtime

- [gVisor](https://github.com/gardener/gardener-extension-runtime-gvisor)
- [Kata Containers](https://github.com/23technologies/gardener-extension-runtime-kata)

### Network Plugin

- [Calico](https://github.com/gardener/gardener-extension-networking-calico)
- [Cilium](https://github.com/gardener/gardener-extension-networking-cilium)

### Generic Extensions

- [ACL](https://github.com/stackitcloud/gardener-extension-acl)
- [Audit (`metal-stack/gardener-extension-audit`)](https://github.com/metal-stack/gardener-extension-audit)
- [Auditing (`gardener/gardener-extension-auditing`)](https://github.com/gardener/gardener-extension-auditing)
- [csi-driver-lvm](https://github.com/metal-stack/gardener-extension-csi-driver-lvm)
- [Envoy Gateway](https://github.com/gardener/gardener-extension-envoy-gateway)
- [Image Rewriter](https://github.com/gardener/gardener-extension-image-rewriter)
- [Minimal Working Example](https://github.com/23technologies/gardener-extension-mwe)
- [Registry Cache](https://github.com/gardener/gardener-extension-registry-cache)
- [S3 Compatible Storage](https://github.com/metal-stack/gardener-extension-backup-s3)
- [Shoot Certificate Service](https://github.com/gardener/gardener-extension-shoot-cert-service)
- [Shoot DNS Service](https://github.com/gardener/gardener-extension-shoot-dns-service)
- [Shoot Falco Service](https://github.com/gardener/gardener-extension-shoot-falco-service)
- [Shoot Flux Service](https://github.com/stackitcloud/gardener-extension-shoot-flux)
- [Shoot Lakom Service](https://github.com/gardener/gardener-extension-shoot-lakom-service)
- [Shoot Networking Filter](https://github.com/gardener/gardener-extension-shoot-networking-filter)
- [Shoot Networking Problem Detector](https://github.com/gardener/gardener-extension-shoot-networking-problemdetector)
- [Shoot OpenID Connect Service](https://github.com/gardener/gardener-extension-shoot-oidc-service)
- [Shoot Rsyslog Relp](https://github.com/gardener/gardener-extension-shoot-rsyslog-relp)
- [Shoot Traefik](https://github.com/gardener/gardener-extension-shoot-traefik)

### Others

- [OIDC Apps Controller](https://github.com/gardener/oidc-apps-controller)

> [!NOTE]
> If you implemented a new extension, please feel free to add it to this list!

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/gardener/gardener/issues) or reach out on [Slack](https://join.slack.com/t/gardener-cloud/shared_invite/zt-33c9daems-3oOorhnqOSnldZPWqGmIBw).

## Learn More!

You can find further resources about our project here:

* Our landing page [gardener.cloud](https://gardener.cloud/)
* Blog posts on kubernetes.io:
    * [Gardener - The Kubernetes Botanist](https://kubernetes.io/blog/2018/05/17/gardener/)
    * [Gardener Project Update](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)
* [GEP-0001 (Gardener Enhancement Proposal) on extensibility](https://github.com/gardener/enhancements/tree/main/geps/0001-gardener-extensibility)
* [Extensibility API documentation](https://github.com/gardener/gardener/tree/master/docs/extensions)
