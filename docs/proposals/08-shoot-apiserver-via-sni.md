---
title: 08 Shoot APIServer via SNI
---

# SNI Passthrough Proxy for kube-apiservers

This GEP tackles the problem that today a single `LoadBalancer` is needed for every single Shoot cluster's control plane.

## Background

When the control plane of a Shoot cluster is provisioned, a dedicated LoadBalancer is created for it. It keeps the entire flow quite easy - the apiserver Pods are running and they are accessible via that LoadBalancer. It's hostnames / IP addresses are used for DNS records like `api.<external-domain>` and `api.<shoot>.<project>.<internal-domain>`. While this solution is simple, it comes with several issues.

## Motivation

There are several problems with the current setu:

- IaaS provider costs. For example, `ClassicLoadBalancer` on AWS costs at minimum 17 USD / month.
- Quotas can limit the amount of LoadBalancers you can get per account / project, limiting the number of clusters you can host under a single account.
- Lack of support for better loadbalancing [algorithms than round-robin](https://www.envoyproxy.io/docs/envoy/v1.10.0/intro/arch_overview/load_balancing/load_balancers#supported-load-balancers).
- Slow cluster provisioning time - depending on the provider, a LoadBalancer provisioning could take quite a while.
- Lower downtime when workload is shuffled in the clusters as the LoadBalancer is Kubernetes-aware.

## Goals

- Only one LoadBalancer is used for all Shoot cluster API servers running in a Seed cluster.
- Out-of-cluster (end-user / robot) communication to the API server is still possible.
- In-cluster communication via the kubernetes master service (IPv4/v6 ClusterIP and the `kubernetes.default.svc.cluster.local`) is possible.
- Client TLS authentication works without intermediate TLS termination (TLS is terminated by `kube-apiserver`).
- Solution should be cloud-agnostic.

## Proposal

### Seed Cluster

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

1. Client requests `Shoot Cluster A` and sets the `Server Name` in the TLS handshake to `api.shoot-a.foo.bar`.
1. This packet goes through the Network LB and it's forwarded to the Proxy server. (this loadbalancer should be a simple Layer-4 TCP proxy)
1. The proxy server reads the packet and see that client requests `api.shoot-a.foo.bar`.
1. Based on its configuration, it maps `api.shoot-a.foo.bar` to `Shoot API Server Cluster A`.
1. It acts as TCP proxy and simply send the data `Shoot API Server Cluster A`.

There are multiple OSS proxies for this case:

- nginx
- HAProxy
- Envoy
- traefik
- [linkerd2-proxy](https://github.com/linkerd/linkerd2-proxy)

To ease integration, it should:

- be configurable via Kubernetes resources.
- not require restarting when configuration changes.
- be fast and with little overhead.

All things considered, [Envoy proxy](http://envoyproxy.io/) is the most fitting solution as it provides all the features Gardener would like (no process reload being the most important one + battle tested in production by various companies).

While building a custom control plane for Envoy is [quite simple](https://github.com/envoyproxy/go-control-plane), an already established solution might be the better path forward. [Istio's Pilot](https://istio.io/docs/concepts/traffic-management/#pilot-and-envoy) is one of the most feature-complete Envoy control plane solutions, as it offers a way to configure edge ingress traffic for Envoy via [Gateway](https://istio.io/docs/reference/config/networking/v1alpha3/gateway/) and [VirtualService](https://istio.io/docs/reference/config/networking/v1alpha3/virtual-service/).

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

In this case, the `internal` and `external` `DNSEntries` should be changed to the Network LoadBalancer's IP.

### In-Cluster Communication to the apiserver

In Kubernetes, the API server is discoverable via the master service (`kubernetes` in `default` namespace). Today, this service can only be of type `ClusterIP` - making in-cluster communication to the API server impossible due to:

- the client doesn't set the `Server Name` in the TLS handshake if it attempts to talk to an IP address. In this case, the TLS handshake reaches the Envoy IngressGateway proxy, but it's rejected by it.
- Kubernetes services can be of type `ExternalName`, but the master service is not supported by [kubelet](https://github.com/gardener/gardener/issues/1135#issuecomment-505317932).
  - even if this is fixed in future Kubernetes versions, this problem still exists for older versions where this functionality is not available.

Another issue occurs when the client tries to talk to the apiserver via the in-cluster DNS. For all Shoot API servers `kubernetes.default.svc.cluster.local` is the same and when a client tries to connect to that API server using that server name. This makes distinction between different in-cluster Shoot clients impossible by the Envoy IngressGateway.

To mitigate this problem an additional proxy must be deployed on every single Node. It does not terminate TLS and sends the traffic to the correct Shoot API Server. This is achieved by:

- the apiserver master service reconciler is started and pointing to the `kube-apiserver`'s Cluster IP in the Seed cluster (e.g. `--advertise-address=10.1.2.3`).
- the proxy runs in the host network of the `Node`.
- the proxy has a sidecar container which:
  - creates a dummy network interface and assigns the `10.1.2.3` to it.
  - removes connection tracking (conntrack) if iptables/nftables is enabled as the IP address is local to the `Node`.
- the proxy listens on the `10.1.2.3` and using the [PROXY protocol](http://www.haproxy.org/download/2.0/doc/proxy-protocol.txt) it sends the data stream to the Envoy ingress gateway (EIGW).
- EIGW listens for PROXY protocol on a dedicated `8443` port. EIGW reads the destination IP + port from the PROXY protocol and forwards traffic to the correct upstream apiserver.

The sidecar is a standalone component. It's possible to transparently change the proxy implementation without any modifications to the sidecar. The simplified flow looks like:

```text
+------------------+                    +----------------+
| Shoot API Server |       TCP          |   Envoy IGW    |
|                  +<-------------------+ PROXY listener |
| Cluster A        |                    |     :8443      |
+------------------+                    +-+--------------+
                                          ^
                                          |
                                          |
                                          |
                                          |
+-----------------------------------------------------------+
                                          |   Single Node in
                                          |   the Shoot cluster
                                          |
                                          | PROXY Protocol
                                          |
                                          |
                                          |
 +---------------------+       +----------+----------+
 |  Pod talking to     |       |                     |
 |  the kubernetes     |       |       Proxy         |
 |  service            +------>+  No TLS termination |
 |                     |       |                     |
 +---------------------+       +---------------------+
```

Multiple OSS solutions can be used:

- haproxy
- nginx

To add a PROXY lister with Istio, several resources must be created - a dedicated `Gateway`, dummy `VirtualService` and `EnvoyFilter` which adds listener filter (`envoy.listener.proxy_protocol`) on `8443` port:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: blackhole
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 8443
      name: tcp
      protocol: TCP
    hosts:
    - "*"

---

apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: blackhole
  namespace: istio-system
spec:
  hosts:
  - blackhole.local
  gateways:
  - blackhole
  tcp:
  - match:
    - port: 8443
    route:
    - destination:
        host: localhost
        port:
          number: 9999 # any dummy port will work

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: proxy-protocol
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
  - applyTo: LISTENER
    match:
      context: ANY
      listener:
        portNumber: 8443
        name: 0.0.0.0_8443
    patch:
      operation: MERGE
      value:
        listener_filters:
        - name: envoy.filters.listener.proxy_protocol
```

For each individual `Shoot` cluster, a dedicated [FilterChainMatch](https://www.envoyproxy.io/docs/envoy/v1.13.0/api-v2/api/v2/listener/listener_components.proto#listener-filterchainmatch) is added. It ensures that only Shoot API servers can receive traffic from this listener:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: <shoot-namespace>
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
  - applyTo: FILTER_CHAIN
    match:
      context: ANY
      listener:
        portNumber: 8443
        name: 0.0.0.0_8443
    patch:
      operation: ADD
      value:
        filters:
        - name: envoy.filters.network.tcp_proxy
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
            stat_prefix: outbound|443||kube-apiserver.<shoot-namespace>.svc.cluster.local
            cluster: outbound|443||kube-apiserver.<shoot-namespace>.svc.cluster.local
        filter_chain_match:
          destination_port: 443
          prefix_ranges:
          - address_prefix: 10.1.2.3 # kube-apiserver's cluster-ip
            prefix_len: 32
```

> **Note:** This additional `EnvoyFilter` can be removed when Istio supports full [L4 matching](https://istio.io/docs/reference/config/networking/virtual-service/#L4MatchAttributes).

A nginx proxy client in the Shoot cluster on every node could have the following configuration:

```conf
error_log /dev/stdout;
stream {
    server {
        listen 10.1.2.3:443;
        proxy_pass api.<external-domain>:8443;
        proxy_protocol on;

        proxy_protocol_timeout 5s;
        resolver_timeout 5s;
        proxy_connect_timeout 5s;
    }
}

events { }
```

### In-Cluster Communication to the apiserver when ExternalName is Supported

Even if in future versions of Kubernetes, the master service of type `ExternalName` is supported, we still have the problem that in-cluster workload can talk to the server via DNS. For this to work, we still need the above mentioned proxy (this time listening on another IP address `10.0.0.2`). An additional change to CoreDNS would be needed:

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

So when a client requests `kubernetes.default.svc.cluster.local`, it'll be sent to the proxy listening on that IP address.

## Future Work

While out of scope of this GEP, several things can be improved:

- Make the sidecar work with eBPF and environments where iptables/nftables are not enabled.

## References

- [One API Server LB per Seed for all Shoots Issue](https://github.com/gardener/gardener/issues/1135)
