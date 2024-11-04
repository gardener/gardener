---
title: Rework apiserver-proxy to drop PROXY protocol
gep-number: 30
creation-date: 2024-10-11
status: implementable
authors:
- "@robinschneider"
- "@knht"
reviewers:
- "@timebertt"
---

# GEP-30: Rework apiserver-proxy to drop PROXY protocol

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
This proposal addresses a critical security vulnerability in the Gardener's API server proxy configuration when used with opaque or intransparent LoadBalancers, particularly in conjunction with the ACL extension. The issue involves the potential for dual proxy protocol headers, which can lead to misrouting, information leakage, and possible unauthorized access attempts. The proposed solution is to implement a custom header for secure routing using HTTP CONNECT (as currently used by the VPN listener), introduce a new port or path for API traffic, and gradually transition to this system through a feature gate.

## Motivation
The current architecture of Gardener, when used with opaque/instransparent LoadBalancer configurations and the ACL extension, can result in a security vulnerability. This vulnerability stems from the addition of multiple proxy protocol headers to network packets, which can lead to:

1. Misrouting of traffic
2. Potential information leakage
3. Possible unauthorized access attempts to incorrect kube-api servers

### Goals
- Eliminate the vulnerability caused by dual proxy protocol headers
- Ensure secure routing of traffic to the correct API server
- Provide a clear migration path for existing Gardener deployments
- Maintain or improve the overall performance and reliability of the system
- Allow `gardener-extension-acl` to be used with proxy protocol on (non-transparent) LoadBalancers

### Non-Goals
- Modifying the core functionality of the ACL extension
- Addressing security issues unrelated to the proxy protocol headers and API server routing
- Reusing existing VPN or tunnel-related infrastructure

## Proposal

The proposed solution involves the following key changes:

- Reconfigure the `istio-ingressgateway` to use the new routing method
    - Introduce a new port/path for API traffic from apiserver-proxy
    - Implement a new custom header for secure API server routing
- Reconfigure the `apiserver-proxy` to use HTTP CONNECT for secure tunneling
- Develop a feature gate to control the rollout of the new routing mechanism
- Provide a phased implementation approach for gradual adoption

## Current Architecture
With the current architecture, the `apiserver-proxy` creates a proxy protocol header, which gets forwarded by the LoadBalancer towards the `istio-ingressgateway`.

A second proxy protocol header is created by the LoadBalancer, which is configured through the ACL extension.
At this point, we need the destination IP address from the `apiserver-proxy` proxy protocol header and the source IP from the LoadBalancer proxy protocol header.

Unfortunately, the `istio-ingressgateway` will read the first proxy protocol header and then overwrites the information with the second proxy protocol header and only uses these source and destination IP addresses.

These source IP addresses will be used for filtering allowed traffic.

Instead of the public IP address from the router *(here 10.1.0.1)* it will allow traffic from the client-pod *(here 10.3.0.1)*. This causes other clients in other shoots with the same IP address to bypass the list of allowed clients by the ACL extension and cause unauthorized requests to the `kube-apiserver`.

![Current State with opaque LB](./assets/30-current-architecture.png "Current State with opaque LB")

## Proposed Changes

1. Custom Header Implementation
    - Introduce a new custom header, e.g. `X-Gardener-API-Route`
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

6. Component Reconfigurations:
    - Modify `istio-ingressgateway` to process the new header and route traffic accordingly
    - Update `apiserver-proxy` to add the new custom header

![Proposed Architecture](./assets/30-proposed-architecture.png "Proposed Architecture")
    
    
## Custom Header Specification

The new custom header will be structured as follows:

```
X-Gardener-API-Route: <routing-information>
```

The routing information will be a concatenated string, joined by pipes, containing the following fields:

```json
{
  "direction": "outbound",
  "port": "8134",
  "subset": null,
  "destination": "kube-apiserver.{{ .namespace }}.svc.cluster.local"
}
```

- `direction`: The direction of traffic.
- `port`: The destination port.
- `subset`: Istio subset/version.
- `destination`: The destination.

Putting this to practice should result in a header formatted like this:

`X-Gardener-API-Route: outbound|8134||kube-apiserver.{{ .namespace }}.svc.cluster.local`

Or formatted in YAML in a potential deployment manifest

```yaml
- name: X-Gardener-API-Route
  value: outbound|8134||kube-apiserver.{{ .namespace }}.svc.cluster.local
```

During the transition period, the system should be able to handle both the new custom header and the existing routing method. This can be controlled via the feature gate.

If the header is missing, malformed, or fails verification, the error should be logged with appropriate details for debugging and return an HTTP 400 (Bad Request) status code. It should never fall back to the old routing method to ensure security.

## HTTP CONNECT Implementation

The `apiserver-proxy` will be reconfigured to use HTTP CONNECT for establishing a secure tunnel to the `kube-apiserver`. This involves:

1. Initiating an HTTP CONNECT request to the new port/path on the `istio-ingressgateway`
2. Including the new custom header in the CONNECT request
3. Establishing a secure tunnel once the CONNECT request is accepted
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
    ...
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: api-routes
  namespace: istio-system
spec:
  ...
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
Controls the enablement of the new routing implementation using HTTP CONNECT and custom headers.

#### Alpha
- **Default:** Disabled
- **Actions:**
    - Introduces new port for HTTP CONNECT
    - Deploys new custom header processing
    - Configures mTLS for new routing path
    - Enables E2E testing of the feature
    - Implementation of all unit and integration tests
    - Documentation completion

#### Beta
- **Default:** Enabled
- New shoots automatically use new implementation
- Existing shoots continue using old implementation

#### GA
- **Default:** Always enabled
- All shoots use new implementation
- Existing shoots automatically migrated during reconciliation

### Feature Gate 2: `APIServerLegacyPortDisable`
Controls the disabling of the legacy port. Can only progress once `APIServerSecureRouting` is GA.

#### Alpha
- **Default:** Disabled
- **Prerequisites:**
    - Feature gate `APIServerSecureRouting` is in GA state
    - All monitored shoots successfully using the new implementation
- **Actions:**
    - Marks port as deprecated
    - Implementation of E2E tests with legacy port disabled

#### Beta
- **Default:** Enabled
- Automatically closes legacy port for new shoots
- Existing shoots prompted for migration during reconciliation

#### GA
- **Default:** Always enabled
- Legacy port completely disabled
- All configuration for legacy port removed

### Testing Strategy
- Feature gates can be switched off per shoot to allow testing of the old implementation
- E2E tests will cover both new and old implementations
- All testing functionality is implemented and validated in alpha phase

## Alternatives

- Implement stricter filtering of proxy protocol headers at the istio-ingress gateway level
- Use a custom LoadBalancer solution that is transparent or aware of the Gardener architecture

In all seriousness, we are not aware of any other or real alternative solution to address this issue.

---
