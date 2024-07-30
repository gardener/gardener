---
title: Shoot Firewall Configuration
weight: 15
---

# Endpoints and Ports of a Shoot Control-Plane

With the [reversed VPN](../operations/reversed-vpn-tunnel.md) tunnel, there are no endpoints with open ports in the shoot cluster required by Gardener.
In order to allow communication to the shoots control-plane in the seed cluster, there are endpoints shared by multiple shoots of a seed cluster.
Depending on the configured zones or [exposure classes](./nessesary_considerations/exposureclasses.md), there are different endpoints in a seed cluster. The IP address(es) can be determined by a DNS query for the API Server URL.
The main entry-point into the seed cluster is the load balancer of the Istio ingress-gateway service. Depending on the infrastructure provider, there can be one IP address per zone.

The load balancer of the Istio ingress-gateway service exposes the following TCP ports:

* **443** for requests to the shoot API Server. The request is dispatched according to the set TLS SNI extension.
* **8443** for requests to the shoot API Server via `api-server-proxy`, dispatched based on the proxy protocol target, which is the IP address of `kubernetes.default.svc.cluster.local` in the shoot.
* **8132** to establish the reversed VPN connection. It's dispatched according to an HTTP header value.

For detailed information, you can check [Control Plane Endpoints and Ports](../development/control-plane-endpoints-and-ports.md).