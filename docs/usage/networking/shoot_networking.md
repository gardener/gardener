---
title: Shoot Networking Configurations
description: Configuring Pod network. Maximum number of Nodes and Pods per Node
---

# Shoot Networking Configurations

This document contains network related information for Shoot clusters.

## Pod Network

A Pod network is imperative for any kind of cluster communication with Pods not started within the Node's host network.
More information about the Kubernetes network model can be found in the [Cluster Networking](https://kubernetes.io/docs/concepts/cluster-administration/networking/) topic.

Gardener allows users to configure the Pod network's CIDR during Shoot creation:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  networking:
    type: <some-network-extension-name> # {calico,cilium}
    pods: 100.96.0.0/16
    nodes: ...
    services: ...
```

> :warning: The `networking.pods` IP configuration is immutable and cannot be changed afterwards. 
> Please consider the following paragraph to choose a configuration which will meet your demands.

One of the network plugin's (CNI) tasks is to assign IP addresses to Pods started in the Pod network.
Different network plugins come with different IP address management (IPAM) features, so we can't give any definite advice how IP ranges should be configured.
Nevertheless, we want to outline the standard configuration.

Information in `.spec.networking.pods` matches the [--cluster-cidr flag](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/) of the Kube-Controller-Manager of your Shoot cluster.
This IP range is divided into smaller subnets, also called `podCIDRs` (default mask `/24`) and assigned to Node objects `.spec.podCIDR`.
Pods get their IP address from this smaller node subnet in a default IPAM setup.
Thus, it must be guaranteed that enough of these subnets can be created for the maximum amount of nodes you expect in the cluster.

_**Example 1**_
```
Pod network: 100.96.0.0/16
nodeCIDRMaskSize: /24
-------------------------

Number of podCIDRs: 256 --> max. Node count 
Number of IPs per podCIDRs: 256
```

With the configuration above a Shoot cluster can at most have **256 nodes** which are ready to run workload in the Pod network.

_**Example 2**_
```
Pod network: 100.96.0.0/20
nodeCIDRMaskSize: /24
-------------------------

Number of podCIDRs: 16 --> max. Node count 
Number of IPs per podCIDRs: 256
```

With the configuration above a Shoot cluster can at most have **16 nodes** which are ready to run workload in the Pod network.

Beside the configuration in `.spec.networking.pods`, users can tune the `nodeCIDRMaskSize` used by Kube-Controller-Manager on shoot creation.
A smaller IP range per node means more `podCIDRs` and thus the ability to provision more nodes in the cluster, but less available IPs for Pods running on each of the nodes.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  kubernetes:
    kubeControllerManager:
      nodeCIDRMaskSize: 24 # (default)
```

> :warning: The `nodeCIDRMaskSize` configuration is immutable and cannot be changed afterwards.

_**Example 3**_
```
Pod network: 100.96.0.0/20
nodeCIDRMaskSize: /25
-------------------------

Number of podCIDRs: 32 --> max. Node count 
Number of IPs per podCIDRs: 128
```

With the configuration above, a Shoot cluster can at most have **32 nodes** which are ready to run workload in the Pod network.

## Reserved Networks

Some network ranges are reserved for specific use-cases in the communication between seeds and shoots.

| IPv  | CIDR                  | Name                         | Purpose                                                                                                                           |
|------|-----------------------|------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| IPv6 | fd8f:6d53:b97a:1::/96 | Default VPN Range            |                                                                                                                                   |
| IPv4 | 240.0.0.0/8           | Kube-ApiServer Mapping Range | Used for the `kubernetes.default.svc.cluster.local` service in a shoot                                                            |
| IPv4 | 241.0.0.0/8           | Seed Pod Mapping Range       | Used for allowing overlapping IPv4 networks between shoot and seed. Requires non-HA control plane. Only used within the vpn pods  |
| IPv4 | 242.0.0.0/8           | Shoot Node Mapping Range     | Used for allowing overlapping IPv4 networks between shoot and seed. Requires non-HA control plane. Only used within the vpn pods  |
| IPv4 | 243.0.0.0/8           | Shoot Service Mapping Range  | Used for allowing overlapping IPv4 networks between shoot and seed. Requires non-HA control plane. Only used within the vpn pods  |
| IPv4 | 244.0.0.0/8           | Shoot Pod Mapping Range      | Used for allowing overlapping IPv4 networks between shoot and seed. Requires non-HA control plane. Only used within the vpn pods  |

> :warning: Do not use any of the CIDR ranges mentioned above for any of the node, pod or service networks.
> Gardener will prevent their creation. Pre-existing shoots using reserved ranges will still work, though it is recommended
> to recreate them with compatible network ranges.

## Overlapping IPv4 Networks between Seed and Shoot
By default, the seed and shoot clusters must have non-overlapping IPv4 network ranges and gardener will enforce disjunct ranges.
However, under certain conditions it is possible to allow overlapping IPv4 network ranges:
1. The shoot cluster must have a non-highly-available VPN, usually implicitly selected by having a non-highly-available control plane.
2. The shoot cluster need be either single-stack IPv4 or dual-stack IPv4/IPv6.
3. The shoot cluster networks don't use the reserved ranges mentioned above.

> **Note:** single-stack IPv6 shoots are usually not affected due to the vastly larger address space. However, 
> Gardener still enforces the non-overlapping condition for IPv6 networks to avoid any potential issues.

If all conditions are met, the seed and shoot clusters can have overlapping (IPv4) network ranges.
The potentially overlapping ranges are mapped to the reserved ranges mentioned above within the VPN network, i.e., double network address translation (NAT) is used .