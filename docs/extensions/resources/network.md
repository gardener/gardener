# Contract: `Network` Resource

Gardener is an open-source project that provides a nested user model. Basically, there are two types of services provided by Gardener to its users:

- Managed: end-users only request a Kubernetes cluster (Clusters-as-a-Service)
- Hosted: operators utilize Gardener to provide their own managed version of Kubernetes (Cluster-Provisioner-as-a-service)

Whether a user is an operator or an end-user, it makes sense to provide choice. For example, for an end-user it might make sense to 
choose a network-plugin that would support enforcing network policies (some plugins does not come with network-policy support by default).
For operators however, choice only matters for delegation purposes, i.e., when providing an own managed-service, it becomes important to also provide choice over which network-plugins to use.

Furthermore, Gardener provisions clusters on different cloud-providers with different networking requirements. For example, Azure does not support Calico overlay networking with IP in IP [1], this leads to the introduction of manual exceptions in static add-on charts which is error prone and can lead to failures during upgrades.

Finally, every provider is different, and thus the network always needs to adapt to the infrastructure needs to provide better performance. Consistency does not necessarily lie in the implementation but in the interface.

## Motivation

Prior to the `Network Extensibility` concept, Gardener followed a mono network-plugin support model (i.e., Calico). Although this seemed to be the easier approach, it did not completely reflect the real use-case.
The goal of the Gardener Network Extensions is to support different network plugins, therefore, the specification for the network resource won't be fixed and will be customized based on the underlying network plugin.

To do so, a `ProviderConfig` field in the spec will be provided where each plugin will define. Below is an example for how to deploy Calico as the cluster network plugin.

## The Network Extensions Resource

Here is what a typical `Network` resource would look-like:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
metadata:
  name: my-network
spec:
  ipFamilies:
  - IPv4
  podCIDR: 100.244.0.0/16
  serviceCIDR: 100.32.0.0/13
  type: calico
  providerConfig:
    apiVersion: calico.networking.extensions.gardener.cloud/v1alpha1
    kind: NetworkConfig
    backend: bird
    ipam:
      cidr: usePodCIDR
      type: host-local
status:
  ipFamilies:
  - IPv4
```

The spec of above resources is divided into two parts (more information can be found at [Using the Networking Calico Extension](https://github.com/gardener/gardener-extension-networking-calico/blob/master/docs/usage/usage.md)):

- global configuration (e.g., podCIDR, serviceCIDR, and type)
- provider specific config (e.g., for calico we can choose to configure a `bird` backend)

> **Note**: Certain cloud-provider extensions might have webhooks that would modify the network-resource to fit into their network specific context. As previously mentioned, Azure does not support IPIP, as a result, the [Azure provider extension](https://github.com/gardener/gardener-extension-provider-azure) implements a [webhook](https://github.com/gardener/gardener-extension-provider-azure/blob/master/pkg/webhook/network/mutate.go) to mutate the backend and set it to `None` instead of `bird`.

## Supporting a New Network Extension Provider

To add support for another networking provider (e.g., weave, Cilium, Flannel) a network extension controller needs to be implemented which would optionally have its own custom configuration specified in the `spec.providerConfig` in the `Network` resource. For example, if support for a network plugin named `gardenet` is required, the following `Network` resource would be created:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
metadata:
  name: my-network
spec:
  ipFamilies:
  - IPv4
  podCIDR: 100.244.0.0/16
  serviceCIDR: 100.32.0.0/13
  type: gardenet
  providerConfig:
    apiVersion: gardenet.networking.extensions.gardener.cloud/v1alpha1
    kind: NetworkConfig
    gardenetCustomConfigField: <value>
    ipam:
      cidr: usePodCIDR
      type: host-local
```

Once applied, the presumably implemented `Gardenet` extension controller would pick the configuration up, parse the `providerConfig`, and create the necessary resources in the shoot.

For additional reference, please have a look at the [networking-calico](https://github.com/gardener/gardener-extension-networking-calico) provider extension, which provides more information on how to configure the necessary charts, as well as the actuators required to reconcile networking inside the `Shoot` cluster to the desired state.

## Supporting `kube-proxy`-less Service Routing

Some networking extensions support service routing without the `kube-proxy` component. This is why Gardener supports disabling of `kube-proxy` for service routing by setting `.spec.kubernetes.kubeproxy.enabled` to `false` in the `Shoot` specification. The implicit contract of the flag is:

*If `kube-proxy` is disabled, then the networking extension is responsible for the service routing.*

The networking extensions need to handle this twofold:

1. During the reconciliation of the networking resources, the extension needs to check whether `kube-proxy` takes care of the service routing or the networking extension itself should handle it. In case the networking extension should be responsible according to `.spec.kubernetes.kubeproxy.enabled` (but is unable to perform the service routing), it should raise an error during the reconciliation. If the networking extension should handle the service routing, it may reconfigure itself accordingly.
1. (Optional) In case the networking extension does not support taking over the service routing (in some scenarios), it is recommended to also provide a validating admission webhook to reject corresponding changes early on. The validation may take the current operating mode of the networking extension into consideration.

## Supporting Migration of `ipFamilies`

To enable the migration from a shoot cluster with single-stack networking to a cluster with dual-stack networking, the `status` field of the `Network` resource includes the `ipFamilies` field. 

This field reflects the currently deployed configuration and is used to verify whether the migration process has been completed successfully. To support the migration from single-stack to dual-stack networking, a network extension provider must ensure that this field is properly maintained and updated during the migration process.

## Related Links

- [1] [Calico overlay networking on Azure](https://docs.tigera.io/calico/latest/networking/configuring/vxlan-ipip#encapsulation-types)
