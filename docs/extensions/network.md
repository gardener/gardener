# Gardener Network Extension

Gardener is an open-source project that provides a nested user model. Basically, there are two types of services provided by Gardener to its users:

- Managed: end-users only request a Kubernetes cluster (Clusters-as-a-Service)
- Hosted: operators utilize Gardener to provide their own managed version of Kubernetes (Cluster-Provisioner-as-a-service)


Whether an operator or an end-user, it makes sense to provide choice. For example, for an end-user it might make sense to 
choose a network-plugin that would support enforcing network policies (some plugins does not come with network-policy support by default). 
For operators however, choice only matters for delegation purposes i.e., when providing an own managed-service, it becomes important to also provide choice over which network-plugins to use.
 
Furthermore, Gardener provisions clusters on different cloud-providers with different networking requirements. For example, Azure does not support Calico Networking [1], this leads to the introduction of manual exceptions in static add-on charts which is error prone and can lead to failures during upgrades.

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
```

The above resources is divided into two parts (more information can be found [here](https://github.com/gardener/gardener-extension-networking-calico/blob/master/docs/usage-as-end-user.md)):

- global configuration (e.g., podCIDR, serviceCIDR, and type)
- provider specific config (e.g., for calico we can choose to configure a `bird` backend)

> **Note**: certain cloud-provider extensions might have webhooks that would modify the network-resource to fit into their network specific context. As previously mentioned, Azure does not support IPIP, as a result, the [Azure provider extension](https://github.com/gardener/gardener-extension-provider-azure) implements a [webhook](https://github.com/gardener/gardener-extension-provider-azure/blob/master/pkg/webhook/network/mutate.go) to mutate the backend and set it to `None` instead of `bird`.

## Supporting a new Network Extension Provider

To add support for another networking provider (e.g., weave, Cilium, Flannel, etc.) a network extension controller needs to be implemented which would optionally have its own custom configuration specified in the `spec.providerConfig` in the `Network` resource. For example, if support for a network plugin named `gardenet` is required, the following `Network` resource would be created:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
metadata:
  name: my-network
spec:
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

Once applied, the presumably implemented `Gardenet` extension controller, would pick the configuration up, parse the `providerConfig` and create the necessary resources in the shoot.

For additional reference, please have a look at the [networking-calico](https://github.com/gardener/gardener-extension-networking-calico) provider extension, which provides more information on how to configure the necessary charts as well as the actuators required to reconcile networking inside the `Shoot` cluster to the desired state.


## References

[1] [Azure support for Calico Networking](https://docs.projectcalico.org/v3.0/reference/public-cloud/azure)
