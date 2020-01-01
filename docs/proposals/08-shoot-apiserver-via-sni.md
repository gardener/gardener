# SNI Passthrough proxy for kube-apiservers

This GEP tackles the problem that today a single `LoadBalancer` is needed for every single Shoot cluster's control plane.

## Background

When the control plane of a Shoot cluster is provisioned, a dedicated LoadBalancer is created for it. It keeps the entire flow quite easy - the apiserver Pods are running and they are accessible via that LoadBalancer. It's hostnames / IP addresses are used for DNS records like `api.<external-domain>` and `api.<shoot>.<project>.<internal-domain>`. While this solution is simple it comes with several issues.

## Motivation

There are several problems with the current setup.

- IaaS provider costs. For example `ClassicLoadBalancer` on AWS costs at minimum 17 USD / month.
- Quotas can limit the amount of LoadBalancers you can get per account / project, limiting the number of clusters you can host under a single account.
- Lack of support for better loadbalancing [algorithms than round-robin](https://www.envoyproxy.io/docs/envoy/v1.10.0/intro/arch_overview/load_balancing/load_balancers#supported-load-balancers).
- Slow cluster provisioning time - depending on the provider a LoadBalancer provisioning could take quite a while.
- Lower downtime when workload is shuffled in the clusters as the LoadBalancer is Kubernetes-aware.

## Goals

- Only one LoadBalancer is used for all Shoot cluster API servers running in a Seed cluster.
- Out-of-cluster (end-user / robot) communication to the API server is still possible.
- In-cluster communication via the kubernetes master service (IPv4/v6 ClusterIP and the `kubernetes.default.svc.cluster.local`) is possible.
- Client TLS authentication works without intermediate TLS termination (TLS is terminated by `kube-apiserver`).
- Solution should be cloud-agnostic.

## Proposal

### Seed cluster

To solve the problem of having multiple `kube-apiservers` behind a single LoadBalancer, an intermediate proxy must be placed between the Cloud-Provider's LoadBalancer and `kube-apiservers`. This proxy is going to choose the Shoot API Server with the help of Server Name Indication. From [wikipedia](https://en.wikipedia.org/wiki/Server_Name_Indication):

> Server Name Indication (SNI) is an extension to the Transport Layer Security (TLS) computer networking protocol by which a client indicates which hostname it is attempting to connect to at the start of the handshaking process. This allows a server to present multiple certificates on the same IP address and TCP port number and hence allows multiple secure (HTTPS) websites (or any other service over TLS) to be served by the same IP address without requiring all those sites to use the same certificate. It is the conceptual equivalent to HTTP/1.1 name-based virtual hosting, but for HTTPS.

A rough diagram of the flow of data:

```text
+-------------------------------+
|                               |
|           Network LB          | (accessible from clients)
|                               |
|                               |
+-------------+-------+---------+                       +------------------+
              |       |                                 |                  |
              |       |            proxy + lb           | Shoot API Server |
              |       |    +-------------+------------->+                  |
              |       |    |                            | Cluster A        |
              |       |    |                            |                  |
              |       |    |                            +------------------+
              |       |    |
     +----------------v----+--+
     |        |               |
   +-+--------v----------+    |                         +------------------+
   |                     |    |                         |                  |
   |                     |    |       proxy + lb        | Shoot API Server |
   |        Proxy        |    +-------------+---------->+                  |
   |                     |    |                         | Cluster B        |
   |                     |    |                         |                  |
   |                     +----+                         +------------------+
   +----------------+----+
                    |
                    |
                    |                                   +------------------+
                    |                                   |                  |
                    |             proxy + lb            | Shoot API Server |
                    +-------------------+-------------->+                  |
                                                        | Cluster C        |
                                                        |                  |
                                                        +------------------+
```

Sequentially:

1. client requests `Shoot Cluster A` and sets the `Server Name` in the TLS handshake to `api.shoot-a.foo.bar`.
1. this packet goes through the Network LB and it's forwarded to the Proxy server. (this loadbalancer should be a simple Layer-4 TCP proxy)
1. the proxy server reads the packet and see that client requests `api.shoot-a.foo.bar`.
1. based on its configuration, it maps `api.shoot-a.foo.bar` to `Shoot API Server Cluster A`.
1. it acts as TCP proxy and simply send the data `Shoot API Server Cluster A`.

There are multiple OSS proxies for this case:

- nginx
- HAProxy
- Envoy
- traefik
- [linkerd2-proxy](https://github.com/linkerd/linkerd2-proxy)

To ease integration it should:

- be configurable via Kubernetes resources
- not require restarting when configuration changes
- be fast and with little overhead

All things considered, [Envoy proxy](http://envoyproxy.io/) is the most fitting solution as it provides all the features Gardener would like (no process reload being the most important one + battle tested in production by various companies).

While building a custom control plane for Envoy is [quite simple](https://github.com/envoyproxy/go-control-plane), an already established solution might be the better path forward. [Istio's Pilot](https://istio.io/docs/concepts/traffic-management/#pilot-and-envoy) is one of the most feature-complete Envoy control plane solutions as it offers a way to configure edge ingress traffic for Envoy via [Gateway](https://istio.io/docs/reference/config/networking/v1alpha3/gateway/) and [VirtualService](https://istio.io/docs/reference/config/networking/v1alpha3/virtual-service/).

The resources which needs to be created per Shoot clusters are the following:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: kube-apiserver-gateway
  namespace: <shoot-namespace>
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 443
      name: tls
      protocol: TLS
    tls:
      mode: PASSTHROUGH
    hosts:
    - api.<external-domain>
    - api.<shoot>.<project>.<internal-domain>
```

and correct `VirtualService` pointing to the correct API server:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: kube-apiserver
  namespace: <shoot-namespace>
spec:
  hosts:
  - api.<external-domain>
  - api.<shoot>.<project>.<internal-domain>
  gateways:
  - kube-apiserver-gateway
  tls:
  - match:
    - port: 443
      sniHosts:
      - api.<external-domain>
      - api.<shoot>.<project>.<internal-domain>
    route:
    - destination:
        host: kube-apiserver.<shoot-namespace>.svc.cluster.local
        port:
          number: 443
```

The resources above configures Envoy to forward the raw TLS data (without termination) to the Shoot `kube-apiserver`.

Updated diagram:

```text
+-------------------------------+
|                               |
|           Network LB          | (accessible from clients)
|                               |
|                               |
+-------------+-------+---------+                       +------------------+
              |       |                                 |                  |
              |       |            proxy + lb           | Shoot API Server |
              |       |    +-------------+------------->+                  |
              |       |    |                            | Cluster A        |
              |       |    |                            |                  |
              |       |    |                            +------------------+
              |       |    |
     +----------------v----+--+
     |        |               |
   +-+--------v----------+    |                         +------------------+
   |                     |    |                         |                  |
   |                     |    |       proxy + lb        | Shoot API Server |
   |    Envoy Proxy      |    +-------------+---------->+                  |
   | (ingress Gateway)   |    |                         | Cluster B        |
   |                     |    |                         |                  |
   |                     +----+                         +------------------+
   +-----+----------+----+
         |          |
         |          |
         |          |                                   +------------------+
         |          |                                   |                  |
         |          |             proxy + lb            | Shoot API Server |
         |          +-------------------+-------------->+                  |
         |   get                                        | Cluster C        |
         | configuration                                |                  |
         |                                              +------------------+
         |
         v                                                  Configure
      +--+--------------+         +---------------------+   via Istio
      |                 |         |                     |   Custom Resources
      |     Pilot       +-------->+   Seed API Server   +<------------------+
      |                 |         |                     |
      |                 |         |                     |
      +-----------------+         +---------------------+
```

In this case the `internal` and `external` `DNSEntries` should be changed to the Network LoadBalancer's IP.

### In-cluster communication to the apiserver

In Kubernetes the API server is discoverable via the master service (`kubernetes` in `default` namespace). Today, this service can only be of type `ClusterIP` - making in-cluster communication to the API server impossible due to:

- the client doesn't set the `Server Name` in the TLS handshake, if it attempts to talk to an IP address. In this case, the TLS handshake reaches the Envoy IngressGateway proxy, but it's rejected by it.
- Kubernetes services can be of type `ExternalName`, but the master service is not supported by [kubelet](https://github.com/gardener/gardener/issues/1135#issuecomment-505317932).
  - even if this is fixed in future Kubernetes versions, this problem still exists for older versions where this functionality is not available.

Another issue occurs when the client tries to talk to the apiserver via the in-cluster DNS. For all Shoot API servers `kubernetes.default.svc.cluster.local` is the same and when a client tries to connect to that API server using that server name. This makes distinction between different in-cluster Shoot clients impossible by the Envoy IngressGateway.

To mitigate this problem an additional proxy must be deployed on every single Node. It does not terminate TLS and sends the traffic to the correct Shoot API Server. This is achieved by:

- the apiserver master service reconciler is stopped (`--endpoint-reconciler-type=none`).
- the proxy runs in the host network of the `Node`.
- the proxy has a sidecar container which:
  - creates a dummy network interface and assigns the master service IP address to it.
  - enables (with iptables) traffic to that IP address (kube-proxy rejects all traffic to `Services` without endpoints) and it removes connection tracking (conntrack) as the IP address is local to the `Node`.
- the proxy needs to send multiple TLS streams over a single TCP connection (to reduce performance overhead).
- the proxy listens on the master service IP address and it sends traffic to the correct API server.

The sidecar is a standalone component. It's possible to transparently change the proxy implementation without any modifications to the sidecar. The simplified flow looks like:

```text
                                              +------------------+
                                              |                  |
                                              | Shoot API Server |
                                              |                  |
                                              | Cluster A        |
                                              |                  |
                                              +------------------+
                                                        ^
                                                        |
                                                        |
                                                        |
                                                        |
  +-----------------------------------------------------------------------+
                                                        |   Shoot Cluster
                                                        |
                                                        |
                                                        |
                                                        |
                                                        |
                                                        |
   +---------------------+                   +---------------------+
   |                     |                   |                     |
   |  Pod talking to     |                   |                     |
   |  the kubernetes     |                   |       Proxy         |
   |  service            +------------------>+  No TLS termination |
   |                     |                   |                     |
   |                     |                   |                     |
   +---------------------+                   +---------------------+
```

Multiple OSS solutions can be used:

- https://github.com/ginuerzh/gost (various transport options)
- https://github.com/jpillora/chisel
- https://github.com/xtaci/kcptun (transport is UDP-based)
- OpenVPN is discarded as the performance is abysmal

`chisel` is the fastest and offers the least latency when testing under different conditions. The architecture is simple:

![overview](https://docs.google.com/drawings/d/1p53VWxzGNfy8rjr-mW8pvisJmhkoLl82vAgctO_6f1w/pub?w=960&h=720)

For the Gardener implementation, the `chisel-client` will run as a `DaemonSet` on the `Shoot` cluster and a dedicated `chisel-server` is provisioned for each `Shoot` control plane. To reduce the amount of records we create, it listens on a dedicated port `9443`.

An updated version of the previous SNI configuration:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: kube-apiserver-gateway
  namespace: <shoot-namespace>
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 443
      name: tls
      protocol: TLS
    tls:
      mode: PASSTHROUGH
    hosts:
    - api.<external-domain>
    - api.<shoot>.<project>.<internal-domain>
  - port:
      number: 9443
      name: tls
      protocol: TLS
    tls:
      mode: PASSTHROUGH
    hosts:
    - api.<shoot>.<project>.<internal-domain>
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: kube-apiserver
  namespace: <shoot-namespace>
spec:
  hosts:
  - api.<external-domain>
  - api.<shoot>.<project>.<internal-domain>
  gateways:
  - kube-apiserver-gateway
  tls:
  - match:
    - port: 443
      sniHosts:
      - api.<external-domain>
      - api.<shoot>.<project>.<internal-domain>
    route:
    - destination:
        host: kube-apiserver.<shoot-namespace>.svc.cluster.local
        port:
          number: 443
  - match:
    - port: 9443
      sniHosts:
      - api.<shoot>.<project>.<internal-domain>
    route:
    - destination:
        host: proxy.<shoot-namespace>.svc.cluster.local
        port:
          number: 443
```

Chisel works by creating logical TCP connections via SSH, but it uses only one TCP connection per client/Shoot Node.
With this approach a Chisel server (with appropriate replica count) can be deployed and it'll serve as a proxy into the cluster for those Nodes.

The given that the master service ip address is `10.0.0.1`, a Pod attempting to communicate with it's API server will do the following:

- `GET https://10.0.0.1/` with it's private and public keys.
- due to the proxy's sidecar, all traffic does to the `chisel-client` listening on `10.0.0.1`.
- `chisel-client` has already opened a TCP connection to the upstream server and it creates a new SSH channel, sending the data to `chisel-server`.
- `chisel-server` is assigned to a specific `kube-apiserver` and sends data to it.

One problem with chisel is the lack of custom certificate authority support in the client, but there is an opened PR to [fix it](https://github.com/jpillora/chisel/pull/129).

> As a fun fact, the fastest proxy (in terms of overhead/latency) is based on the new [QUIC transport protocol](https://tools.ietf.org/html/draft-ietf-quic-transport-24). This protocol is used as the transport protocol for HTTP3 and supports multiplexing, multiple streams and TLSv1.3. The proxy + client is only several hundreds LoC written for this tunneling feature. The only downside is that it's UDP-based and support for UDP is lacking (different OpenStack deployments may not support it). Support for QUIC in Envoy is also [coming soon™](https://github.com/envoyproxy/envoy/issues/2557).

### In-cluster communication to the apiserver when ExernalName is supported

Even if in future versions of Kubernetes, the master service of type `ExternalName` is supported, we still have the problem that in-cluster workload can talk to the server via DNS. For this to work we still need the above mentioned proxy (this time listening on another IP address `10.0.0.2`). An additional change to CoreDNS would be needed:

```text
default.svc.cluster.local.:8053 {
    file kubernetes.default.svc.cluster.local
}

.:8053 {
    errors
    health
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods insecure
        upstream
        fallthrough in-addr.arpa ip6.arpa
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}
```

The content of the `kubernetes.default.svc.cluster.local` is going to be:

```text
$ORIGIN default.svc.cluster.local.

@	30 IN	SOA local. local. (
        2017042745 ; serial
        1209600    ; refresh (2 hours)
        1209600    ; retry (1 hour)
        1209600    ; expire (2 weeks)
        30         ; minimum (1 hour)
        )

  30 IN NS local.

kubernetes     IN A     10.0.0.2
```

So when a client requests `kubernetes.default.svc.cluster.local`, it'll be send to the proxy listening on that IP address.

## Future work

While out of scope of this GEP, several things can be improved:

- The Shoot proxy can be replaced with better 2-way tunneling solution removing the need for LoadBalancer in the Shoot cluster for VPN.
- The Shoot proxy can be replaced with QUIC-based implementation.
- Make the sidecar work with eBPF and environments where iptables/nftables are not enabled.

## References

- https://github.com/gardener/gardener/issues/1135
