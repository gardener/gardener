---
title: Self-Hosted Shoot Exposure
gep-number: 36
creation-date: 2025-12-05
status: implementable
authors:
- "@timebertt"
reviewers:
- "@rfranzke"
- "@ScheererJ"
---

# GEP-36: Self-Hosted Shoot Exposure

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)

## Summary

This proposal introduces a standardized mechanism for exposing the API server of self-hosted shoot clusters with managed infrastructure (see [GEP-28](28-self-hosted-shoot-clusters.md#managed-infrastructure)), e.g., using a load balancer of the underlying infrastructure provider or other strategies.
It defines a new extension resource, `SelfHostedShootExposure`, and describes how control plane exposure can be configured, managed, and reconciled, enabling external access to shoot clusters in a flexible and provider-agnostic way.

## Motivation

API servers of hosted shoot clusters can be accessed externally via a DNS name (`api.<Shoot.spec.dns.domain>`).
The DNS record points to the load balancer of the istio ingress gateway of the hosting seed cluster (see [GEP-08](08-shoot-apiserver-via-sni.md)).

For convenience and consistency, self-hosted shoot clusters should also be accessible externally via a DNS name using the same pattern.
However, in self-hosted shoot clusters, the control plane is hosted within the shoot cluster itself, and there is no hosting seed cluster to provide the necessary exposure mechanism, i.e., no istio ingress gateway.
In case of a self-hosted shoot cluster with unmanaged infrastructure, Gardener expects the operator to manually set up the necessary DNS record pointing to the control plane nodes or an external load balancer.

For self-hosted shoot clusters with managed infrastructure, Gardener reuses many existing components (e.g., extensions and machine-controller-manager) for managing the infrastructure, control plane components, and machines.
Similarly, it should provide a standardized way to externally expose the API server of self-hosted shoot clusters using existing components.
However, there is no mechanism in these existing components to handle the exposure of a Kubernetes control plane in the desired way.
Hence, this proposal introduces a new extension resource for this particular purpose.

### Goals

- Enable external access to the API server of self-hosted shoot clusters with managed infrastructure
- Provide a flexible, extension-based mechanism for control plane exposure by defining a new Gardener extension resource
- Support multiple exposure strategies (e.g., cloud load balancer or DNS) to fit different use cases
- Allow extensions to implement provider-specific/custom logic for exposing shoot control planes

### Non-Goals

- Exposing shoot clusters with unmanaged infrastructure
- Defining the full lifecycle or implementation details of all possible exposure strategies

## Proposal

### `Shoot` API Changes

To specify which exposure mechanism should be used for the control plane of a self-hosted shoot cluster, the `Shoot` API is extended as follows:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  provider:
    type: local
    workers:
    - name: control-plane
      controlPlane:
        exposure: # either `extension` or `dns` or null
          extension:
            type: local # defaults to `.spec.provider.type`, but could also be different
            providerConfig: {} # *runtime.RawExtension
          dns: {}
```

In the control plane worker pool, a new optional `exposure` field is added.
It can be used to specify that the control plane should be exposed using a `SelfHostedShootExposure` extension (via the `extension` field, see [The `SelfHostedShootExposure` Extension Resource](#selfhostedshootexposure-extension-resource)) or directly via DNS (via the `dns` field, see [DNS-Based Exposure](#dns-based-control-plane-exposure)).

The `extension.type` field specifies which `SelfHostedShootExposure` extension should be used, defaulting to the value of `spec.provider.type` if not set.
Additional configuration for the extension can be provided via the optional `extension.providerConfig` field.

### `SelfHostedShootExposure` Extension Resource

If the new `exposure.extension` field is set, `gardenadm init` (for initial bootstrapping) or the `gardenlet` (after [connecting the shoot to a garden](28-self-hosted-shoot-clusters.md#gardenadm-connect)) creates/updates a `SelfHostedShootExposure` object in the `kube-system` namespace (similar to the other self-hosted shoot extension objects).
This resource instructs the corresponding extension controller to manage the necessary resources for exposing the control plane of the self-hosted shoot cluster and allows the extension to report the resulting ingress addresses, e.g.:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: SelfHostedShootExposure
metadata:
  name: example
  namespace: kube-system
spec:
  # extensionsv1alpha1.DefaultSpec
  type: stackit
  providerConfig: {} # *runtime.RawExtension

  # control plane endpoints that should be exposed
  endpoints:
  - nodeName: example-control-plane
    addresses: # []corev1.NodeAddress
    - address: 172.18.0.2
      type: InternalIP
    - address: example-control-plane
      type: Hostname
    port: 443
  # - ... more endpoints for HA control planes
status:
  # extensionsv1alpha1.DefaultStatus
  observedGeneration: 1
  lastOperation:
    type: Reconcile
    state: Succeeded
  
  # endpoints of the exposure mechanism
  ingress: # []corev1.LoadBalancerIngress
  - ip: 1.2.3.4
  - hostname: external.load-balancer.example.com
```

The `spec` includes the default set of fields included in all extension resources like `type` and `providerConfig` (see [GEP-01](01-extensibility.md)).
Additionally, the `spec.endpoints` list contains all healthy control plane node addresses that should be exposed.
Each endpoint includes the node name, a list of addresses (based on the `Node.status.addresses` list) and the port of the API server (usually `443`).

The `status` includes the default fields included in all extension resources like `observedGeneration` and `lastOperation`.
Additionally, the `status.ingress` list contains resulting addresses of the exposure mechanism, e.g., the IPs or hostnames of a load balancer.
The `status.ingress` field has the same type as `Service.status.loadBalancer.ingress` as it will be used as the source for the values of the corresponding `DNSRecord` – similar to how the status of the istio ingress gateway service is used for the `DNSRecord` values of hosted shoot clusters.

As usual, `gardenadm`/`gardenlet` will wait for the object to be reconciled successfully and update the (already existing) `DNSRecord` object's `.spec.values[]` with the addresses out of the reported `.status.ingress[]`.
IP addresses are preferred over hostnames when updating the `DNSRecord`.

#### Extension Controller Interface

The extension controller implementing `SelfHostedShootExposure` must reconcile resources for exposing the control plane and update `.status.ingress` with the resulting addresses.
A new controller for the `SelfHostedShootExposure` resource will be added to the extension library, similar to other existing extension controllers.
The corresponding `Actuator` interface implemented by the extension looks as follows:

```go
type Actuator interface {
  // Reconcile creates/reconciles all resources for the exposure of the self-hosted shoot control plane.
  Reconcile(context.Context, *extensionsv1alpha1.SelfHostedShootExposure, *extensionscontroller.Cluster) ([]corev1.LoadBalancerIngress, error)
  // Delete removes all resources that were created for the exposure of the self-hosted shoot control plane.
  Delete(context.Context, *extensionsv1alpha1.SelfHostedShootExposure, *extensionscontroller.Cluster) error
}
```

When reconciling a `SelfHostedShootExposure` object, the extension controller returns the resulting list of `LoadBalancerIngress` addresses that will be stored in `.status.ingress` (implemented in the controller of the extension library).

#### Examples of Possible Extension Implementations

A typical provider extension can implement the `SelfHostedShootExposure` resource by creating a load balancer on the underlying infrastructure and configuring it to forward traffic to the control plane nodes specified in `.spec.endpoints`.
I.e., the extension controller would ensure a load balancer and the correct target pool similar to the `Service` controller of a cloud-controller-manager.

For infrastructures or scenarios where creating a load balancer is not possible or desired, an alternative implementation of the `SelfHostedShootExposure` resource can install a software-defined load balancer (e.g., [kube-vip](https://kube-vip.io/) or [MetalLB](https://metallb.io/)) on the control plane nodes themselves (e.g., via a `DaemonSet`) – possibly in combination with provider-specific infrastructure resources (e.g., external IPs and NICs).
E.g., in an OpenStack environment with layer 2 connectivity but without load balancer support, the extension controller could create a floating IP (external IP) and a port (NIC) in the shoot's network, install kube-vip on the control plane nodes, and configure kube-vip to advertise the port's IP as a virtual IP via ARP.

In [provider-local](../extensions/provider-local.md), the `SelfHostedShootExposure` controller can create a `Service` of type `LoadBalancer` in underlying kind cluster and configure it to forward traffic to the control plane nodes.
The `LoadBalancer` service in the kind cluster would simulate a cloud provider load balancer by forwarding traffic from the host machine on a specific IP (bound to the loopback device) to the control plane machines hosted as pods in the kind cluster.

### DNS-Based Control Plane Exposure

As an alternative to using a `SelfHostedShootExposure` extension, the control plane of a self-hosted shoot cluster can also be exposed directly via DNS.
In this case, no additional API objects or infrastructure resources for exposing the control plane are created, and the control plane nodes' addresses are passed directly to the `DNSRecord` object's `.spec.values[]` by `gardenadm`/`gardenlet`.

While this approach is simpler and requires no additional extension controller, it has some limitations compared to using a `SelfHostedShootExposure` extension.
Most notably, the DNS record updates (e.g., when control plane machines are rolled out) might be delayed due to DNS caching.
Also, there is no load balancing mechanism in front of the control plane nodes, so clients need to handle multiple addresses themselves.
Furthermore, if the control plane nodes are not exposed externally (i.e., do not have external IPs or hostnames), the control plane still cannot be accessed from outside the cluster.

### gardenlet Controller for Updating Control Plane Endpoints

The gardenlet responsible for the self-hosted shoot cluster (deployed by `gardenadm connect`) runs a new controller that watches the `Node` objects of the control plane worker pool.

If the shoot uses a `SelfHostedShootExposure` extension, the controller updates the `SelfHostedShootExposure.spec.endpoints[]` list with the `.status.addresses[]` of all healthy control plane nodes.
Once the `SelfHostedShootExposure` object has been reconciled successfully, the controller updates the corresponding `DNSRecord` object's `.spec.values[]` with the addresses reported by the extension in `SelfHostedShootExposure.status.ingress[]` if necessary.

If the shoot uses DNS-based control plane exposure, the controller directly updates the `DNSRecord` object's `.spec.values[]` with the addresses of all healthy control plane nodes.

#### Future Optimization

Not all extensions implementing `SelfHostedShootExposure` require continuously updated control plane endpoints.
E.g., an extension using kube-vip only needs to create the infrastructure resources once and then kube-vip will elect a leader and dynamically advertise the virtual IP from one of the healthy control plane nodes.
To omit unnecessary API requests made by the gardenlet controller, a future enhancement could be to add a field to the `ControllerRegistration` API that allows extensions to specify if they need a continuously updated `SelfHostedShootExposure.spec.endpoints` list or not.

## Alternatives

### Service of Type `LoadBalancer`

Instead of introducing a new extension resource, `gardenadm`/`gardenlet` could manage a `Service` of type `LoadBalancer` in the self-hosted shoot cluster that forwards traffic to the control plane nodes.
However, this approach has several drawbacks:

- Services of type `LoadBalancer` are explicitly not designed for exposing control planes and typically exclude control plane nodes from traffic (see [kubernetes/kubernetes#65618](https://github.com/kubernetes/kubernetes/issues/65618), [kubeadm](https://github.com/kubernetes/kubernetes/blob/4e94e70dcad423e9f59f12ac5a048d2137d20e86/cmd/kubeadm/app/constants/constants.go#L276-L278), [KEP-1143](https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/1143-node-role-labels#service-load-balancer))
- Services of type `LoadBalancer` would include worker nodes in the target pool by default, which is not desired for control plane exposure
- Using `externalTrafficPolicy: Local` to restrict the target pool to control plane nodes would result in the loss of connectivity in case kube-proxy is not running on the control plane nodes (even if the API server is available)
- Other exposure strategies (e.g., software-defined load balancers or purely DNS-based exposure) cannot be implemented using a `Service` of type `LoadBalancer`
