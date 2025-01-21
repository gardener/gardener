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

The proposed solution involves the following key changes:

- Reconfigure the `istio-ingressgateway` to use the new routing method
    - Introduce a new port/path for API traffic from apiserver-proxy
    - Implement a new custom header for secure API server routing
- Reconfigure the `apiserver-proxy` to use HTTP CONNECT for secure tunneling
- Develop a feature gate to control the rollout of the new routing mechanism
- Provide a phased implementation approach for gradual adoption

## Proposed Changes

1. Custom Header Implementation
    - Introduce a new custom header `X-Gardener-API-Route`
    - Define the structure and encoding of the header value to include necessary routing information

2. New Port/Path
    - Implement a new port or path on the `istio-ingressgateway` for the new routing mechanism
    - Ensure this new route is separate from any existing VPN or tunnel infrastructure

3. HTTP CONNECT Reconfiguration:
    - Reconfigure the `apiserver-proxy` to use HTTP CONNECT for establishing a secure tunnel
    - This will replace the current TCP proxy protocol method
    - The HTTP CONNECT method will be used on the newly implemented port/path

4. Feature Gate
    - Implement a feature gate to control the rollout of the new routing mechanism
    - Define stages for the feature gate: alpha, beta, and stable

5. Phased Implementation
    - Phase 1: Add the new port/path and custom header processing
    - Phase 2: Gradually reconfigure shoots to use the new routing method
    - Phase 3: Deprecate and remove the old routing method

![Proposed Architecture](./assets/30-proposed-architecture-http-connect.png "Proposed Architecture")
![Proposed Architecture](./assets/30-proposed-architecture-tls.png "Proposed Architecture")
    
    
## Custom Header Specification

The new custom header will be structured as follows:

```
X-Gardener-API-Route: <routing-information>
```

The routing information will be a concatenated string, joined by pipes, containing the following fields:

- `direction`: The direction of traffic.
- `port`: The destination port.
- `subset`: Istio subset/version.
- `destination`: The destination.

Putting this to practice should result in a header formatted like this:

`X-Gardener-API-Route: outbound|8134||kube-apiserver.foobar.svc.cluster.local`

During the transition period, the system should be able to handle both the new custom header and the existing routing method. This can be controlled via the feature gate.

If the header is missing, malformed, or fails verification, the error should be logged with appropriate details for debugging and return an HTTP 400 (Bad Request) status code. It should never fall back to the old routing method to ensure security.

## HTTP CONNECT Implementation

The `apiserver-proxy` will be reconfigured to use HTTP CONNECT for establishing a secure tunnel to the `kube-apiserver`. This involves:

1. Initiating an HTTP CONNECT request to the new port/path on the `istio-ingressgateway`
2. Including the new custom header in the CONNECT request
3. Creating a secure connection over the established tunnel once the CONNECT request is accepted
4. Forwarding the original API server request through this secure tunnel

### Technical Implementation Details

The `apiserver-proxy` will be required to be updated to use HTTP CONNECT. This should involve modifying its configuration and potentially its code. One example of how the configuration might look like after reusing its Envoy tunneling config:

```yaml
...
- domains: 
  - api.*
  name: gardener-api-route
  routes:
  - match:
      connect_matcher: {}
    route:
     cluster_header: X-Gardener-API-Route
     upgrade_configs:
     - connect_config: {}
       upgrade_type: CONNECT
...
```

This solution would be reusing existing Envoy filtering on the `apiserver-proxy` pod and simply exchange its Proxy Protocol configuration for the new proposed header.

### Istio IngressGateway Configuration

Similarly, it would be required to configure the Istio IngressGateway to accept HTTP CONNECT requests and route them appropriately. This could e.g. be achieved utilizing Istio Custom Resources (Gateway, VirtualService):

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: api-gateway
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
    ...
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: api-routes
  namespace: istio-system
spec:
  ...
  gateways:
  - api-gateway
  http:
  - match:
    - method:
        exact: CONNECT
      headers:
        X-Gardener-API-Route:
          regex: ".*"
    ...
```

This configuration sets up the Istio IngressGateway to accept HTTPS traffic and route HTTP CONNECT requests with the proposed custom header to the `kube-apiserver`.

### EnvoyFilter for Custom Header Processing

To process the proposed custom header and make routing decisions based on it, it will be required to additionally add and/or modify an EnvoyFilter. Again, existing configuration may be reused adjusted to this new use case to keep convention parity within the code base.

```yaml
{{- if eq .Values.vpn.enabled true -}}
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: reversed-vpn-auth-server
  namespace: {{ .Release.Namespace }}
spec:
  selector:
{{ .Values.labels | toYaml | indent 4 }}
  servers:
  - hosts:
    - reversed-vpn-auth-server.garden.svc.cluster.local
    port:
      name: tls-tunnel
      number: 8134
      protocol: HTTP
{{- end }}
```

### Implementation Steps

1. Apply the Istio Gateway and VirtualService configurations.
2. Deploy the EnvoyFilter for custom header processing.
3. Update any relevant Istio authorization policies to allow the new traffic flow.
4. Update the `apiserver-proxy` code or configuration to use HTTP CONNECT and include the custom header.
5. Deploy the updated `apiserver-proxy` configuration.

## Phased Rollout

The implementation will be controlled by two distinct feature gates that handle different aspects of the solution. Each feature gate can be disabled per shoot to allow testing of the old implementation via E2E tests.

### Feature Gate 1: `APIServerSecureRouting`

Controls whether shoots use the new secure routing implementation with HTTP CONNECT and custom headers.

**Rollout Plan:**
- Initially introduced as disabled by default
- Once stability is proven in production environments, enabled by default for new shoots
- Existing shoots will retain their previous setting until explicitly migrated
- When fully proven, the feature gate will be removed and the functionality will become permanent

### Feature Gate 2: `APIServerLegacyPortDisable`

Controls whether the legacy port is available. This feature gate can only be enabled after `APIServerSecureRouting` has been fully rolled out.

**Rollout Plan:**
- Introduced as disabled by default
- Once all shoots have migrated to secure routing, enabled by default for new shoots
- Existing shoots will be notified to migrate during reconciliation
- When migration is complete, the feature gate will be removed and the legacy port will be permanently disabled

## Alternatives

We are not aware of any other alternative solution to address this issue, as requiring a transparent Load Balancer for Gardener is no solution.
