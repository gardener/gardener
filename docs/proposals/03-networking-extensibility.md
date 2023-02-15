# Networking Extensibility

Currently, Gardener follows a mono network-plugin support model (i.e., Calico). Although this can seem to be the more stable approach, it does not completely reflect the real use-case. This proposal brings forth an effort to add an extra level of customizability to Gardener networking.

## Motivation

Gardener is an open-source project that provides a nested user model. Basically, there are two types of services provided by Gardener to its users:

- **Managed**: users only request a Kubernetes cluster (Clusters-as-a-Service)
- **Hosted**: users utilize Gardener to provide their own managed version of Kubernetes (Cluster-Provisioner-as-a-service)

For the first set of users, the choice of network plugin might not be so important, however, for the second class of users (i.e., Hosted) it is important to be able to customize networking based on their needs.

Furthermore, Gardener provisions clusters on different cloud-providers with different networking requirements. For example, Azure does not support Calico Networking [1], this leads to the introduction of manual exceptions in static add-on charts which is error prone and can lead to failures during upgrades.

Finally, every provider is different, and thus the network always needs to adapt to the infrastructure needs to provide better performance. Consistency does not necessarily lie in the implementation but in the interface.

## Gardener Network Extension

The goal of the Gardener Network Extensions is to support different network plugin, therefore the specification for the network resource won't be fixed and will be customized based on the underlying network plugin. To do so, a `NetworkConfig` field in the spec will be provided where each plugin will be defined. Below is an example for deploying Calico as the cluster network plugin.


### Long Term Spec
```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
metadata:
  name: calico-network
  namespace: shoot--core--test-01
spec:
  type: calico
  clusterCIDR: 192.168.0.0/24
  serviceCIDR:  10.96.0.0/24
  providerConfig:
    apiVersion: calico.extensions.gardener.cloud/v1alpha1
    kind: NetworkConfig
    ipam:
      type: host-local
      cidr: usePodCIDR
    backend: bird
    typha:
      enabled: true
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
  providerStatus:
    apiVersion: calico.extensions.gardener.cloud/v1alpha1
    kind: NetworkStatus
    components:
      kubeControllers: true
      calicoNodes: true
    connectivityTests:
      pods: true
      services: true
    networkModules:
      arp_proxy: true
    config:
      clusterCIDR: 192.168.0.0/24
      serviceCIDR:  10.96.0.0/24
      ipam:
        type: host-local
        cidr: usePodCIDR
```


### First Implementation (Short Term)

As an initial implementation, the network plugin type will be specified by the user, e.g. Calico (without further configuration in the 
provider spec). This will then be used to generate the `Network` resource in the seed. The Network operator will pick it up, and apply the 
configuration based on the `spec.cloudProvider` specified directly to the shoot or via the Gardener resource manager (still in the works). 

The `cloudProvider` field in the spec is just an initial catalyst but not meant to stay long-term. In the future, 
the network provider configuration will be customized to match the best needs of the infrastructure.
 
Here is how the simplified initial spec would look like: 

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Network
metadata:
  name: calico-network
  namespace: shoot--core--test-01
spec:
  type: calico
  cloudProvider: {aws,azure,...}
status:
  observedGeneration: 2
  lastOperation: ...
  lastError: ...
 ``` 
 
 
## Functionality

The network resource need to be created early-on during cluster provisioning. Once created, the Network operator residing in every seed will create all the necessary networking resources and apply them to the shoot cluster.

The status of the Network resource should reflect the health of the networking components, as well as additional tests if required.

## References

[1] [Azure support for Calico Networking](https://docs.projectcalico.org/v3.0/reference/public-cloud/azure)