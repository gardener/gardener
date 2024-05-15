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
> [!IMPORTANT]
> **TODO:** *Describe the current architecture to drive the point home*

With the current architecture, the `apiserver-proxy` creates a proxy protocol header, which gets forwarded by the LoadBalancer towards the `istio-ingressgateway`.
A second proxy protocol header is created by the LoadBalancer, which is configured through the ACL extension.
At this point, we need the destination IP address from the `apiserver-proxy` proxy protocol header and the source IP from the LoadBalancer proxy protocol header.
Unfortunately, the `istio-ingressgateway` will read the first proxy protocol header and then overwrites the information with the second proxy protocol header and only uses these source and destination IP addresses.
These source IP addresses will be used for filtering allowed traffic.
Instead of the public IP address from the router *(here 10.1.0.1)* it will allow traffic from the client-pod *(here 10.3.0.1)*. This causes other clients in other shoots with the same IP address to bypass the list of allowed clients by the ACL extension and cause unauthorized requests to the `kube-apiserver`.

![Current State with opaque LB](https://hackmd.io/_uploads/B1HcURNx1x.png)


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

![image](https://hackmd.io/_uploads/S17YU4_lyl.png)


## Custom Header Specification

The new custom header will be structured as follows:

```
X-Gardener-API-Route-V1: <encoded-routing-information>
```

> [!NOTE]
> **TODO:** *Discuss details on the encoding method, included information, and any security measures like signing or encryption*

The encoded routing information will be a Base64-encoded JSON string containing the following fields:

```json
{
  "dst": "10.4.0.1",
  "shoot": "shoot-name",
  "namespace": "garden-project",
  "timestamp": 1635724800
  ...
}
```

- `dst`: The destination IP (`kube-apiserver` IP).
- `shoot`: The name of the shoot cluster.
- `namespace`: The namespace of the shoot cluster in the seed.
- `timestamp`: Unix timestamp to prevent replay attacks.
- ... ? :-)

The encoding process constructs the JSON object with all fields and encodes the entire JSON string using Base64. Similarly, the decoding process should Base64 decode the header value and parse the JSON object. In addition the timestamp should be verified to be within an acceptable range.

During the transition period, the system should be able to handle both the new custom header and the existing routing method. This can be controlled via the feature gate.

If the header is missing, malformed, or fails verification, the error should be logged with appropriate details for debugging and return an HTTP 400 (Bad Request) status code. It should never fall back to the old routing method to ensure security.

## HTTP CONNECT Implementation

The `apiserver-proxy` will be reconfigured to use HTTP CONNECT for establishing a secure tunnel to the `kube-apiserver`. This involves:

1. Initiating an HTTP CONNECT request to the new port/path on the `istio-ingressgateway`
2. Including the new custom header in the CONNECT request
3. Establishing a secure tunnel once the CONNECT request is accepted
4. Forwarding the original API server request through this secure tunnel

### Technical Implementation Details

> [!Note]
> **TODO:** Go over technical implementation and discuss with team

The `apiserver-proxy` will require to be updated to use HTTP CONNECT. This should involve modifying its configuration and potentially its code. One example of how the configuration might look like

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: apiserver-proxy-config
  namespace: kube-system
data:
  config.yaml: |
    upstreamUrl: https://istio-ingressgateway.istio-system:8443
    connectMethod: HTTP_CONNECT
    connectPath: /api/v1/namespaces/kube-system/services/kube-apiserver:https/proxy
    customHeaders:
      - name: X-Gardener-API-Route-V1
        valueFrom:
          fieldRef:
            fieldPath: metadata.annotations['gardener.cloud/api-route']
```

This ConfigMap would be mounted into the apiserver-proxy pod and used to configure its behavior.

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
        X-Gardener-API-Route-V1:
          regex: ".*"
    ...
    ...
```

This configuration sets up the Istio IngressGateway to accept HTTPS traffic and route HTTP CONNECT requests with the proposed custom header to the `kube-apiserver`.

### EnvoyFilter for Custom Header Processing

To process the proposed custom header and make routing decisions based on it, it will be required to additionally add an EnvoyFilter. It might be required to add Lua scripts to process the proposed custom header, extract the routing information, and set the appropriate headers for upstream routing

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: api-route-header-processor
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      listener:
        filterChain:
          filter:
            name: "envoy.filters.network.http_connection_manager"
    patch:
      operation: INSERT_BEFORE
      value:
        name: envoy.lua
        typed_config:
          "@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"
          inlineCode: |
            function envoy_on_request(request_handle)
              local header = request_handle:headers():get("X-Gardener-API-Route-V1")
              if header then
                -- Base64 decode and JSON parse the header
                local routing_info = parse_routing_info(header)
                -- Set headers for upstream routing
                request_handle:headers():add("X-Forwarded-For", routing_info.src)
                request_handle:headers():add("X-Envoy-Original-Dst-Host", routing_info.dst)
              end
            end

            function parse_routing_info(header)
              -- Implement Base64 decoding and JSON parsing here
              -- Return a table with src and dst fields
            end
```

### Implementation Steps

1. Apply the Istio Gateway and VirtualService configurations.
2. Deploy the EnvoyFilter for custom header processing.
3. Update any relevant Istio authorization policies to allow the new traffic flow.
4. Update the `apiserver-proxy` code or configuration to use HTTP CONNECT and include the custom header.
5. Deploy the updated `apiserver-proxy` configuration.

## Feature Gate Implementation

Two distinct feature gates should be implemented that control different aspects of the implementation. Their progression will be based on specific success criteria and dependencies rather than fixed timelines.

### Feature Gate 1: `APIServerSecureRouting`
This feature gate controls the enablement of the new routing implementation using HTTP CONNECT and custom headers.

This feature gate should progress through the following stages:

1. Alpha
    - **Default:** Disabled
    - **Graduation requirements:**
        - All unit tests passing
        - Integration tests for new routing path successful
        - E2E tests showing no regression in existing functionality
        - Successful deployment in development environment
        - Performance metrics meeting baseline requirements
        - Documentation completed
    - **Actions:**
        - Introduces new port for HTTP CONNECT
        - Deploys new custom header processing
        - Configures mTLS for new routing path
2. Beta
    - **Default:** Enabled
    - **Graduation requirements:**
        - No critical issues reported in alpha
        - E2E tests consistently passing
        - Performance metrics meeting or exceeding baseline
        - Majority of test clusters successfully using new implementation
        - No new security vulnerability identified
    - **Actions:**
        - Automatically configures new shoots to use new implementation
        - Existing shoots continue using old implementation
3. Stable
    - **Default:** Always enabled
    - **Graduation requirements:**
        - No critical issues reported in beta
        - All E2E tests passing consistently
        - Performance metrics stable and satifsactory
        - Possible security audits completed successfully
        - All test clusters successfully using new implementation
        - Documentation fully updated and verified
    - **Actions:**
        - All new shoots use new implementation
        - Existing shoots automatically migrated during reconciliation
4. Removed
    - **Requirements:**
        - Feature has been stable for sufficient time
        - All shoots successfully migrated to new implementation
        - No reported issues or regressions
    - **Actions:**
        - Remove feature gate configuration
        - Clean up related legacy code

### Feature Gate 2: `APIServerLegacyPortDisable`

This feature gate controls the disabling of the legacy port. It can only progress once `APIServerSecureRouting` is stable.


Similarly, this feature gate should progress through the following stages:

1. Alpha
    - **Default:** Disabled
    - **Prerequisites:**
        - Feature gate `APIServerSecureRouting` is in stable state
        - All monitored shoots successfully using the new implementation
    - **Graduation requirements:**
        - No traffic detected on legacy port in test environments
        - Successful E2E tests with legacy port disabled
        - Documentation updated
    - **Actions:**
        - Marks port as deprecated

2. Beta
    - **Default:** Enabled
    - **Graduation requirements:**
        - No critical issues reported in alpha
        - E2E tests passing consistently with legacy port disabled
        - Amount of traffic still using the legacy port is negligible
        - No production impact reported
        - All shoots in test environments successfully operating without legacy port
    - **Actions:**
        - Automatically closes legacy port for new shoots
        - Existing shoots prompted for migration during reconciliation

3. Stable
    - **Default:** Always enabled
    - **Graduation requirements:**
        - No traffic on legacy port whatsoever
        - All shoots successfully using new implementation
        - No issues reported in beta
        - E2E tests consistently passing
        - Possible security audits completed successfully
        - Documentation fully updated and verified
    - **Actions:**
        - Legacy port completely disabled
        - All configuration for legacy port removed

4. Removed
    - **Requirements:**
        - Feature has been stable for sufficient time
        - No shoots using legacy port
        - No reported issues on regressions
    - **Actions:**
        - Remove feature gate configuration
        - Remove all legacy port related code and configurations

## Alternatives

- Implement stricter filtering of proxy protocol headers at the istio-ingress gateway level
- Use a custom LoadBalancer solution that is transparent or aware of the Gardener architecture

We are not aware of any other or real alternative solution to address this issue.

---
