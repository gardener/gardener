---
title: Rework apiserver-proxy to drop proxy protocol
gep-number: 30
creation-date: 2024-10-11
status: implementable
authors:
- "@robinschneider"
- "@knht"
- "@timebertt"
reviewers:
- "@ScheererJ"
---

# GEP-30: Rework apiserver-proxy to drop proxy protocol

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Current Architecture](#current-architecture)
- [Proposed Changes](#proposed-changes)
- [Custom Header Specification](#custom-header-specification)
- [HTTP CONNECT Implementation](#http-connect-implementation)
    - [Technical Implementation Details](#technical-implementation-details)
    - [Istio IngressGateway Configuration](#istio-ingressgateway-configuration)
    - [EnvoyFilter for Custom Header Processing](#envoyfilter-for-custom-header-processing)
    - [Implementation Steps](#implementation-steps)
- [Feature Gate Implementation](#feature-gate-implementation)
    - [Feature Gate 1: APIServerSecureRouting](#feature-gate-1-apiserversecurerouting)
    - [Feature Gate 2: APIServerLegacyPortDisable](#feature-gate-2-apiserverlegacyportdisable)
    - [Configuration Example](#configuration-example)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)

## Summary

This proposal reworks the API server proxy (originally introduced in [GEP-08](08-shoot-apiserver-via-sni.md)) to use [HTTP CONNECT requests](https://datatracker.ietf.org/doc/html/rfc7231#section-4.3.6) (i.e., HTTP proxy) instead of the [proxy protocol](https://www.haproxy.org/download/3.2/doc/proxy-protocol.txt) for connecting in-cluster clients on the shoot side to the corresponding API server on the seed side.
With this, the API server proxy uses the same network infrastructure and protocol to connect to the shoot control plane as the shoot's VPN client (see [GEP-14](14-reversed-cluster-vpn.md)).

The rework allows more scenarios like using the [ACL extension](https://github.com/stackitcloud/gardener-extension-acl) with opaque (non-transparent) LoadBalancers on the seed side that rely on the proxy protocol themselves to preserve the client's IP.

## Motivation

Since [GEP-08](08-shoot-apiserver-via-sni.md) introduced shared LoadBalancers for shoot control planes on the seed side, clients need to indicate which control plane they want to connect to through the LoadBalancer.
The Envoy proxy in the Istio ingress gateway receives the traffic from the shared LoadBalancer and is responsible routing traffic to the indicated control plane.
For this, Gardener currently uses different protocols based on the connection type:

- When connecting to a shoot's API server directly, this is done using TLS SNI (Server Name Indication). I.e., the destination API server is indicated by the hostname in the SNI header of the TLS handshake.
- When connecting to a shoot's API server via the `kubernetes` Service (fallback to the previous protocol for in-cluster clients), the SNI header is set to the same value (`kubernetes.default.svc.cluster.local`) on all shoots and cannot be used to indicate the destination API server. Therefore, the API server proxy handles traffic on this service and prepends a proxy protocol header with a shoot-specific destination IP to indicate the destination.
- When connecting to a shoot's VPN server, the shoot VPN client sends an HTTP CONNECT request to the shared LoadBalancer and indicates the destination by adding the `Reversed-VPN` HTTP header with the Envoy cluster string as a value (e.g., `outbound|1194||vpn-seed-server-0.shoot--foo--bar.svc.cluster.local`). I.e., it uses the ingress gateway as an HTTP proxy. In contrast to usual HTTP proxies, the target in the CONNECT request line is discarded.

Note that in all cases the payload (HTTP request or OpenVPN tunnel) is end-to-end encrypted even if it is tunneled via an unencrypted HTTP connection.

Shoot owners can use the [ACL extension](https://github.com/stackitcloud/gardener-extension-acl) for restricting traffic to the control plane based on client IPs – on all three of the described connection types.
In seed setups where only opaque LoadBalancers are available, the Gardener operator needs to configure the LoadBalancer to use the proxy protocol to preserve the original client IP.
With the proxy protocol, the original client IP is lost and the ACL extension cannot restrict the traffic as configured.

Restricting control plane traffic in such setups works for traffic using the TLS SNI and the HTTP CONNECT protocol.
However, this doesn't work for traffic using the proxy protocol (API server proxy) because it contains two proxy protocol headers and Envoy only allows using the information from the last header.
Because the last header is the one added by the API server proxy (indicating the destination), traffic is routed correctly to the desired destination API server.
However, the original client IP from the first proxy protocol header (added by the LoadBalancer) is lost and replaced by the client IP connecting to the API server proxy (typically a pod IP).
In short, the ACL extension cannot restrict traffic using the proxy protocol if an opaque LoadBalancer is used on the seed side.

In addition to supporting this use case, reworking the API server proxy to use HTTP CONNECT instead of proxy protocol removes one the connection protocols and reduces complexity.

### Goals

- allow [gardener-extension-acl](https://github.com/stackitcloud/gardener-extension-acl) to work with opaque LoadBalancers using proxy protocol
- reduce complexity by removing one protocol for connections to the shoot control plane
- reuse existing network infrastructure (e.g., existing ingress gateway ports)
  - opening new LoadBalancer ports could require manual actions and shoot owner alignment
- share the network infrastructure for both the API server proxy and VPN connection path
- implement a migration path for existing shoot clusters

### Non-Goals

- change the core architecture of the ACL extension
- change the functionality of the API server proxy

## Proposal

### Reconfiguring the API Server Proxy

The API server proxy's Envoy config is changed to stop adding proxy protocol headers and start using HTTP CONNECT as a tunneling mechanism instead.

Essentially, the config will be changed like this (assuming a shoot `shoot--foo--bar`):

```diff
 static_resources:
   listeners:
   - name: kube_apiserver
     filter_chains:
     - filters:
       - name: envoy.filters.network.tcp_proxy
         typed_config:
           "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
           cluster: kube_apiserver
+          tunneling_config:
+            hostname: "api.bar.foo.internal.local.gardener.cloud:443"
+            headers_to_add:
+            - header:
+                key: X-Gardener-Destination
+                value: "outbound|443||kube-apiserver.shoot--foo--bar.svc.cluster.local"

# ...

 clusters:
 - name: kube_apiserver
   load_assignment:
     cluster_name: kube_apiserver
     endpoints:
     - lb_endpoints:
       - endpoint:
           address:
             socket_address:
               address: api.bar.foo.internal.local.gardener.cloud
-               port_value: 8443
+               port_value: 8132
-  transport_socket:
-    name: envoy.transport_sockets.upstream_proxy_protocol
-    typed_config:
-      "@type": type.googleapis.com/envoy.extensions.transport_sockets.proxy_protocol.v3.ProxyProtocolUpstreamTransport
-      config:
-        version: V2
-      transport_socket:
-        name: envoy.transport_sockets.raw_buffer
-        typed_config:
-          "@type": type.googleapis.com/envoy.extensions.transport_sockets.raw_buffer.v3.RawBuffer
```

The TCP proxy filter of the listener is extended with a `tunneling_config` that configures the proxy to tunnel the TCP payload over HTTP CONNECT (see the [Envoy reference](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/tcp_proxy/v3/tcp_proxy.proto#envoy-v3-api-field-extensions-filters-network-tcp-proxy-v3-tcpproxy-tunneling-config)).
The `hostname` field is sent in the CONNECT request to the upstream proxy (ingress gateway).
This value is discarded, but required in the config.
Additionally, the custom header `X-Gardener-Destination` is added to the CONNECT request to indicate the destination API server.
We don't reuse the `Reversed-VPN` header which fulfills the same purpose to avoid confusion.
However, the value of the `X-Gardener-Destination` header follows the same format (i.e., the Envoy cluster string format):

```text
outbound|443||kube-apiserver.<technical-shoot-id>.svc.cluster.local
```

In the corresponding upstream cluster section of the config, the destination port to connect to on istio-ingressgateway is changed from 8443 (`proxy`) to 8132 (`tls-tunnel`).
This configures the API server proxy to reuse the existing VPN infrastructure on the seed side.
Finally, the `transport_socket` is removed which disables adding the proxy protocol headers.

With this, a connection is established as follows:

1. An in-cluster client (e.g., pod) opens a TLS connection to `kubernetes.default.svc.cluster.local`.
2. The Envoy process in the API server proxy pod on the same node sends an HTTP CONNECT request to the API server domain of the shoot, i.e., to the ingress gateway on the corresponding seed.
3. The ingress gateway discards the target from the HTTP request line and opens a TCP connection to the upstream cluster indicated by the `X-Gardener-Destination` header, i.e., in-cluster service of the shoot cluster.
4. The TLS payload is proxied from the in-cluster client via the API server proxy and the ingress gateway to the shoot API server.

### Reconfiguring the Istio Ingress Gateway

By configuring the API server proxy to connect to the existing VPN port on the Istio ingress gateway, we can reuse most of the existing VPN network infrastructure on the seed side without changes.
The existing `EnvoyFilter` `reversed-vpn` is changed to handle the new use case as follows:

```diff
 apiVersion: networking.istio.io/v1alpha3
 kind: EnvoyFilter
 metadata:
   name: reversed-vpn
 spec:
   configPatches:
   - applyTo: NETWORK_FILTER
     match:
       context: GATEWAY
       listener:
         filterChain:
           filter:
             name: envoy.filters.network.http_connection_manager
         portNumber: 8132
     patch:
       operation: MERGE
       value:
         name: envoy.filters.network.http_connection_manager
         typed_config:
           '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
           route_config:
             virtual_hosts:
             - domains: 
               - api.*
               name: reversed-vpn
               routes:
               - match:
                   connect_matcher: {}
+                  headers:
+                  - name: Reversed-VPN
+                    string_match:
+                      safe_regex: '^outbound\|1194\|\|vpn-seed-server(-[0-4])?\..*\.svc\.cluster\.local$'
                 route:
                   cluster_header: Reversed-VPN
                   upgrade_configs:
                   - connect_config: {}
                     upgrade_type: CONNECT
+              - match:
+                  connect_matcher: {}
+                  headers:
+                  - name: X-Gardener-Destination
+                    string_match:
+                      # see https://regex101.com/r/m0ZAAj/1
+                      safe_regex: '^outbound\|(1194\|\|vpn-seed-server(-[0-4])?|443\|\|kube-apiserver)\..*\.svc\.cluster\.local$'
+                route:
+                 cluster_header: X-Gardener-Destination
+                 upgrade_configs:
+                 - connect_config: {}
+                   upgrade_type: CONNECT
               - match:
                   headers:
                   - invert_match: true
                     name: :method
                     string_match:
                       exact: CONNECT
                   prefix: /
                 redirect:
                   https_redirect: true
                   port_redirect: 443
-                typed_per_filter_config:
-                  envoy.filters.http.ext_authz:
-                    '@type': type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthzPerRoute
-                    disabled: true

# ...

-  - applyTo: HTTP_FILTER
-    match:
-      context: GATEWAY
-      listener:
-        filterChain:
-          filter:
-            name: envoy.filters.network.http_connection_manager
-            subFilter:
-              name: envoy.filters.http.router
-        portNumber: 8132
-    patch:
-      filterClass: AUTHZ
-      operation: INSERT_BEFORE
-      value:
-        name: envoy.filters.http.ext_authz
-        typed_config:
-          '@type': type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
-          grpc_service:
-            envoy_grpc:
-              cluster_name: outbound|9001||reversed-vpn-auth-server.garden.svc.cluster.local
```

The existing route matcher in the `reversed-vpn` virtual host is extended to additionally check the `Reversed-VPN` header.
The header's value must match the given regex, so that only VPN servers can be specified as the proxy destination.
Then, a new route is added to the same virtual host that matches and validates the new `X-Gardener-Destination` header but also allows shoot API servers as valid destinations.
As before, all unmatched CONNECT requests are redirected to port 443.

Finally, the external authorization server configuration is removed, because the route matchers fulfill the same functionality but without an externally developed component (see the [ext-authz-server](https://github.com/gardener/ext-authz-server) repository).
With the removal of the `ext_authz` filter, we can also remove the explicit disablement of the filter in the route for unmatched requests.

With this, the 8132 port of the ingress gateway acts as a generic HTTP proxy and allows clients to open an HTTP tunnel to selected seed services (shoot API servers and VPN servers).
An individual HTTP CONNECT request is handled as follows:

1. The HTTP connection manager on the gateway port 8132 matches requests by the `api.*` host header.
2. It then matches the CONNECT type and the content of the `X-Gardener-Destination` header. It only matches requests that indicate a valid destination (shoot API server or VPN server).
3. If the request is matched, the gateway opens a connection to the specified destination and proxies the raw TCP payload from the client over this connection.

### Unifying the HTTP Proxy Infrastructure

Once the API server proxy and VPN connection use the same protocol, we want to unify the network infrastructure for handling these HTTP CONNECT requests on the seed side.
With this, we want to ensure that all related components have intuitive names to avoid confusion and that the names and implementation will match potential future connections using the same protocol.
For this, we ensure the following:

- the VPN connection is switched to the new `X-Gardener-Destination` header and the old `Reversed-VPN` header is dropped
- both API server proxy and shoot VPN client connect to the same port on the ingress gateway named `http-proxy`
  - we reuse the previous `proxy` port 8443 once it is no longer used for the old API server proxy config (see [Rollout Plan](#rollout-plan))
- all related Istio objects like `Gateway`, `EnvoyFilter`, etc. are named `http-proxy` 

### Rollout Plan

The implementation will be controlled by two distinct feature gates that handle different aspects of the solution. Each feature gate can be disabled per shoot to allow testing of the old implementation via E2E tests.

#### Feature Gate 1: `APIServerSecureRouting`

Controls whether shoots use the new secure routing implementation with HTTP CONNECT and custom headers.

**Rollout Plan:**
- Initially introduced as disabled by default
- Once stability is proven in production environments, enabled by default for new shoots
- Existing shoots will retain their previous setting until explicitly migrated
- When fully proven, the feature gate will be removed and the functionality will become permanent

#### Feature Gate 2: `APIServerLegacyPortDisable`

Controls whether the legacy port is available. This feature gate can only be enabled after `APIServerSecureRouting` has been fully rolled out.

**Rollout Plan:**
- Introduced as disabled by default
- Once all shoots have migrated to secure routing, enabled by default for new shoots
- Existing shoots will be notified to migrate during reconciliation
- When migration is complete, the feature gate will be removed and the legacy port will be permanently disabled

## Alternatives

We are not aware of any other alternative solution to address this issue, as requiring a transparent LoadBalancer for Gardener is no solution.

## Appendix

### Visualization of the Current Architecture

![Current Architecture](assets/30-current-architecture.png)

### Visualization of the Proposed Architecture

![Proposed Architecture](assets/30-proposed-architecture.png)
