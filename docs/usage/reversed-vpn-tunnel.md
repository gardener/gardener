---
title: Reversed VPN Tunnel
---

# Reversed VPN Tunnel Setup and Configuration 

The Reversed VPN Tunnel is enabled by default.
A highly available VPN connection is automatically deployed in all shoots that configure an HA control-plane.

## Reversed VPN Tunnel

In the first VPN solution, connection establishment was initiated by a VPN client in the seed cluster.
Due to several issues with this solution, the tunnel establishment direction has been reverted.
The client is deployed in the shoot and initiates the connection from there. This way, there is no need to deploy a special purpose
loadbalancer for the sake of addressing the data-plane, in addition to saving costs, this is considered the more secure alternative.
For more information on how this is achieved, please have a look at the following [GEP](../proposals/14-reversed-cluster-vpn.md).

Connection establishment with a reversed tunnel:

`APIServer --> Envoy-Proxy | VPN-Seed-Server <-- Istio/Envoy-Proxy <-- SNI API Server Endpoint <-- LB (one for all clusters of a seed) <--- internet <--- VPN-Shoot-Client --> Pods | Nodes | Services`

## Configurable VPN Network

The CIDR of the VPN network can be configured using the `Seed.spec.networks.vpn` field.
The field configures the CIDR for all VPN tunnels of shoots running on the given seed.
It defaults to `192.168.123.0/24` for `IPv4` seeds and `fd8f:6d53:b97a:1::/120` for `IPv6` seeds.
The field is mutable and changing it leads to a temporary VPN disconnect during the next reconciliation of all shoots on this seed.

Similar to the other seed networks (see `Seed.spec.networks`), the configured VPN network must not overlap with any other seed network.
Also, shoot networks (see `Shoot.spec.networking`) must not overlap with any seed network including the VPN network.
This is required for packets to be routed without ambiguity in all components of the seed/shoot architecture.
Migrating the control plane of a shoot to a seed cluster with a different VPN network configuration works as long as network disjointedness is still fulfilled.

With the requirement for disjoint seed and shoot networks, Gardener users are limited in choosing network ranges for their shoots.
Gardener operators might want to use coherent ranges for their seed networks including the VPN network to give users more freedom of choice and make the limitations simpler to reason about.
For this, configuring a non-default VPN network on the seed-level allows Gardener operators to use for example the [IANA-Reserved IPv4 Shared Address Space](https://datatracker.ietf.org/doc/html/rfc6598#section-7) (`100.64.0.0/10`) for all seed networks (the ones that users can't choose for their shoots).

## High Availability for Reversed VPN Tunnel

Shoots which define `spec.controlPlane.highAvailability.failureTolerance: {node, zone}` get an HA control-plane, including a
highly available VPN connection by deploying redundant VPN servers and clients. 

Please note that it is not possible to move an open connection to another VPN tunnel. Especially long-running
commands like `kubectl exec -it ...` or `kubectl logs -f ...` will still break if the routing path must be switched 
because either VPN server or client are not reachable anymore. A new request should be possible within seconds.

### HA Architecture for VPN

Establishing a connection from the VPN client on the shoot to the server in the control plane works nearly the same
way as in the non-HA case. The only difference is that the VPN client targets one of two VPN servers, represented by two services 
`vpn-seed-server-0` and `vpn-seed-server-1` with endpoints in pods with the same name.
The VPN tunnel is used by a `kube-apiserver` to reach nodes, services, or pods in the shoot cluster.
In the non-HA case, a kube-apiserver uses an HTTP proxy running as a side-car in the VPN server to address
the shoot networks via the VPN tunnel and the `vpn-shoot` acts as a router.
In the HA case, the setup is more complicated. Instead of an HTTP proxy in the VPN server, the kube-apiserver has
additional side-cars, one side-car for each VPN client to connect to the corresponding VPN server.
On the shoot side, there are now two `vpn-shoot` pods, each with two VPN clients for each VPN server.
With this setup, there would be four possible routes, but only one can be used. Switching the route kills all
open connections. Therefore, another layer is introduced: link aggregation, also named [bonding](https://www.kernel.org/doc/Documentation/networking/bonding.txt).
In Linux, you can create a network link by using several other links as slaves. Bonding here is used with
active-backup mode. This means the traffic only goes through the active sublink and is only changed if the active one
becomes unavailable. Switching happens in the bonding network driver without changing any routes. So with this layer, 
vpn-seed-server pods can be rolled without disrupting open connections.

![VPN HA Architecture](images/vpn-ha-architecture.png)

With bonding, there are 2 possible routing paths, ensuring that there is at least one routing path intact even if
one `vpn-seed-server` pod and one `vpn-shoot` pod are unavailable at the same time.

As it is not possible to use multi-path routing, one routing path must be configured explicitly.
For this purpose, the `path-controller` script is running in another side-car of the kube-apiserver pod.
It pings all shoot-side VPN clients regularly every few seconds. If the active routing path is not responsive anymore,
the routing is switched to the other responsive routing path.

![Four possible routing paths](images/vpn-ha-routing-paths.png)

For general information about HA control-plane, see [GEP-20](../proposals/20-ha-control-planes.md). 
