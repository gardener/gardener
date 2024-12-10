---
title: Utilize API Server Network Proxy to Invert Seed-to-Shoot Connectivity
gep-number: 11
creation-date: 2020-04-20
status: removed/dropped
authors:
- "@zanetworker"
reviewers:
- "@mvladev"
---

# Utilize API Server Network Proxy to Invert Seed-to-Shoot Connectivity

- [Utilize API Server Network Proxy to Invert Seed-to-Shoot Connectivity](#utilize-api-server-network-proxy-to-invert-seed-to-shoot-connectivity)
  - [Problem](#problem)
  - [Proposal](#proposal)
    - [API Server Network Proxy](#api-server-network-proxy)
  - [Challenges](#challenges)
    - [Prometheus to Shoot Connectivity](#prometheus-to-shoot-connectivity)
      - [Possible Solutions](#possible-solutions)
      - [Port-Forwarder Sidecar](#port-forwarder-sidecar)
      - [Proxy Client Sidecar](#proxy-client-sidecar)
      - [Proxy Sub-Resource](#proxy-sub-resource)
    - [Proxy-Server Loadbalancer Sharing and Re-Advertising](#proxy-server-loadbalancer-sharing-and-re-advertising)
      - [Possible Solution](#possible-solution)
    - [Summary](#summary)

## Problem

Gardener's architecture for Kubernetes clusters relies on having the control-plane (e.g., kube-apiserver, kube-scheduler, kube-controller-manager) and the data-plane  (e.g., kube-proxy, kubelet) of the cluster residing in separate places, this provides many benefits but poses some challenges, especially when API-server to system components communication is required. This problem is solved today in Gardener by [making use of OpenVPN](https://github.com/gardener/vpn) to establish a VPN connection from the seed to the shoot. To do so, the following steps are required:

- Create a Loadbalancer service on the shoot.
- Add a sidecar to the API server pod which knows the address of the newly created Loadbalancer.
- Establish a connection over the internet to the VPN Loadbalancer.
- Install additional iptables rules that would redirect all the IPs of the shoot (i.e., service, pod, node CIDRs) to the established VPN tunnel.


There are, however, quite a few problems with the above approach. Some of them are:

- Every shoot would require an additional loadbalancer, this accounts for additional overhead in terms of both costs and troubleshooting efforts.
- Private access use-cases would not be possible without having a seed residing in the same private domain as a hard requirement. For example, have a look at [this issue](https://github.com/gardener/gardener-extension-provider-gcp/issues/56).
- Providing a public endpoint to access components in the shoot poses a security risk.


## Proposal

There are multiple ways to tackle the directional connectivity issue mentioned above, one way would be to invert the connection between the API server and the system components, i.e., instead of having the API server side-car establish a tunnel, we would have an agent residing in the shoot cluster initiate the connection itself. This way we don't need a Loadbalancer for every shoot and from the security perspective, there is no ingress from outside, only controlled egress.

We want to replace this:

`APIServer | VPN-seed ---> internet ---> LB --> VPN-Shoot (4314) --> Pods | Nodes | Services`

With this:

`APIServer <-> Proxy-Server <--- internet <--- Proxy-Agent --> Pods | Nodes | Services`


### API Server Network Proxy

To solve this issue, we can utilize the [apiserver-network-proxy](https://github.com/kubernetes-sigs/apiserver-network-proxy) upstream implementation, which provides a reference implementation for a reverse streaming server. The way it works is as follows:

- The proxy agent connects to the proxy server to establish a sticky connection.
- Traffic to the proxy server (residing in the seed) then gets re-directed to the agent (residing in the shoot), which forwards the traffic to in-cluster components.

The initial motivation for the apiserver-network-proxy project is to get rid of provider-specific implementations that reside in the API-server (e.g., SSH), but it turns out that
it has other interesting use-cases such as data-plane connection decoupling, which is the main use-case for this proposal.

Starting with **Kubernetes 1.18**, it's possible to make use of an `--egress-selector-config-file` flag, this helps point the API-server to traffic hook points based on traffic direction. For example, in the config below the API server would have to forward all cluster related traffic (e.g., logs, port-forward, exec) to the **proxy-server**, which then knows how to forward traffic to the shoot. For the rest of the traffic, e.g., API server to ETCD or other control-plane components, `direct` is used which means legacy routing method, i.e., bypass the proxy.
```yaml
  egress-selector-configuration.yaml: |-
    apiVersion: apiserver.k8s.io/v1alpha1
    kind: EgressSelectorConfiguration
    egressSelections:
    - name: cluster
      connection:
        proxyProtocol: httpConnect
        transport:
          tcp:
            url: https://proxy-server:8131
    - name: master
      connection:
        proxyProtocol: direct
    - name: etcd
      connection:
        proxyProtocol: direct
```

## Challenges

### Prometheus to Shoot Connectivity
One challenge remains to completely eliminate the need for a VPN connection. In today's Gardener setup, each control-plane has a Prometheus instance that directly scrapes cluster components such as CoreDNS, Kubelets, cadvisor, etc. This works because in addition to the VPN side car attached to the API server pod, we have another one attached to prometheus which knows how to forward traffic to these endpoints. Once the VPN is eliminated, it is required to find other means to forward traffic to these components.

#### Possible Solutions

There are currently two ways to solve this problem:

- Attach a port-forwarder side-car to prometheus.
- Utilize the proxy subresource on the API server.

#### Port-Forwarder Sidecar

With this solution, each prometheus instance would have a side-car that has the kubeconfig of the shoot cluster, and which establishes a port-forward connection to the endpoints residing in the shoot.

There are a many problems with this approach:

- The port-forward connection is not reliable.
- The connection would break if the API server instance dies.
- Requires an additional component.
- Would need to expose every pod / service via port-forward.


```console
Prom Pod (Prometheus -> Port-forwarder) <-> APIServer -> Proxy-server <--- internet <--- Proxy-Agent --> Pods | Nodes | Services
```

#### Proxy Client Sidecar

Another solution would be to implement a proxy-client as a sidecar for every component that wishes to communicate with the shoot cluster. For this to work, means to re-direct / inject that proxy to handle the component's traffic is necessary (e.g., additional IPtable rules).

```console
Prometheus Pod (Prometheus -> Proxy) <-> Proxy-Server <--- internet <--- Proxy-Agent --> Pods | Nodes | Services
```

The problem with this approach is that it requires an additional sidecar (along with traffic redirection) to be attached to every client that wishes to communicate with the shoot cluster, this can cause:

- Additional maintenance efforts (extra code).
- Other side-effects (e.g., if istio sidecar injection is enabled).

#### Proxy Sub-Resource

Kubernetes supports proxying requests to nodes, services, and pod endpoints in the shoot cluster. This proxy connection can be utilized for scraping the necessary endpoints in the shoot.

This approach requires less components and is more reliable than the port-forward solution, however, it relies on having the API server supporting proxied connection for the required endpoints.

```console
Prometheus  <-> APIServer <-> Proxy-Server <--- internet <--- Proxy-Agent --> Pods | Nodes | Services
```

As simple as it is, it has a downside that it relies on the availability of the API server.

### Proxy-Server Loadbalancer Sharing and Re-Advertising

With the proxy-server in place, we need to provide means to enable the proxy-agent in the shoot to establish the connection with the server. As a result, we need to provide a public endpoint through which this channel of communication can be established, i.e., we need a Loadbalancer(s).


#### Possible Solution

Using a Loadbalancer / proxy server would not make sense, since this is a pain-point we are trying to eliminate in the first-place, doing so just moves the costs to the control-plane. A possible solution is to communicate over a shared loadbalancer in the seed, similar to what has been proposed in the [SNI Passthrough Proxy for kube-apiservers](./08-shoot-apiserver-via-sni.md) proposal, this way we can prevent the extra-costs for load-balancers.

With this in mind, we still have other pain-points, namely:

- Advertising Loadbalancer public IPs to the shoot.
- Directing the traffic to the corresponding shoot proxy-server.

For advertising the Loadbalancer IP, a DNS entry can be created for the proxy loadbalancer (or re-use the DNS entry for the SNI proxy), along with necessary certificates, which is then used to connect to the loadbalancer. At this point we can decide on either one of the two approaches:

1. One Proxy / API server with a shared loadbalancer.
2. Use one proxy server for all agents.

In the first case, we will probably need a proxy for the proxy-server that knows how to direct traffic to the correct proxy server based on the corresponding shoot cluster. In the second case, we don't need another proxy if the proxy server is cluster-aware, i.e., can pool and identify connections coming from the same cluster and peer them with the correct API. Unfortunately, the second case is not supported today.

### Summary

- API server proxy can be utilized to invert the connection (only for clusters >= 1.18, for older clusters the old VPN solution will remain).
- This is achieved by utilizing the `--egress-selector-config-file` flag on the api-server.
- For monitoring endpoints, the proxy subresources would be the preferable methods to go, but in the future we can also support sidecar proxies that can communicate with the proxy-server.
- For directing traffic to the correct proxy-server, we will re-use the SNI proxy along with the load-balancer from [the shoot API server via SNI GEP](./08-shoot-apiserver-via-sni.md).


