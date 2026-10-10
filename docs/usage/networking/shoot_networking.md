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

For IPv6 (both dual-stack and IPv6 single-stack clusters), the equivalent setting is `nodeCIDRMaskSizeIPv6`.
Whether this field has any effect depends on the infrastructure: on infrastructures that perform per-node IPv6 prefix delegation
(e.g. OpenStack with DHCPv6-PD), the infrastructure assigns node prefixes directly and kube-controller-manager's node-ipam is
bypassed, so `nodeCIDRMaskSizeIPv6` is ignored. On infrastructures where kube-controller-manager manages CIDR allocation,
the field controls the per-node subnet size and matters for cluster capacity.

For the latter case, Gardener does **not** default `nodeCIDRMaskSizeIPv6` to a fixed value, because the usable range depends on
the infrastructure-allocated IPv6 pod CIDR. If the field is left unset, Gardener derives the effective per-node mask from the
actual IPv6 pod CIDR as `podCIDRMaskSize + 16` (capped at `/124`). For a `/64` pod CIDR this results in `/80`, i.e. up to
65,536 nodes. The `+16` is the maximum difference Kubernetes' node-ipam allows for IPv6, so this choice maximizes the possible
node count while still leaving an enormous address space per node.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  kubernetes:
    kubeControllerManager:
      nodeCIDRMaskSizeIPv6: 80 # optional; derived from the IPv6 pod CIDR when unset
```

The derived value is not written back to the spec. You can see the input it is derived from in `.status.networking.pods`.
Like `nodeCIDRMaskSize`, `nodeCIDRMaskSizeIPv6` is immutable: it can only be set at shoot creation or in the update that migrates
a cluster to dual-stack (see [Dual-Stack Network Migration](./dual-stack-networking-migration.md)). Once the cluster is already
dual-stack, the field can no longer be changed.

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
IPv4 or dual-stack shoot clusters can have overlapping network ranges with their seed cluster. 
Gardener will disentangle the overlapping ranges by mapping them to reserved ranges within the VPN network using double network address translation (NAT).
Notice that none of the reserved ranges will show up outside the VPN network, i.e., they are not routable in the seed or shoot cluster.

> **Note:** single-stack IPv6 shoots are usually not affected due to their vastly larger address space. However, 
> Gardener still enforces the non-overlapping condition for IPv6 networks to avoid any potential issues.
