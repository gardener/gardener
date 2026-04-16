# Topology-Aware Traffic Routing

## Motivation

The enablement of [highly available shoot control-planes](../usage/high-availability/shoot_high_availability.md) requires multi-zone seed clusters. A garden runtime cluster can also be a multi-zone cluster. The topology-aware routing is introduced to reduce costs and to improve network performance by avoiding the cross availability zone traffic, if possible. The cross availability zone traffic is charged by the cloud providers and it comes with higher latency compared to the traffic within the same zone. The topology-aware routing feature enables topology-aware routing for `Service`s deployed in a seed or garden runtime cluster. For the clients consuming these topology-aware services, `kube-proxy` favors the endpoints which are located in the same zone where the traffic originated from. In this way, the cross availability zone traffic is avoided.

## How it works

The topology-aware routing feature relies on the Kubernetes features [`TopologyAwareHints`](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/) or [`ServiceTrafficDistribution`](https://kubernetes.io/docs/reference/networking/virtual-ips/#traffic-distribution) based on the runtime cluster's Kubernetes versions.

For Kubernetes versions >= 1.31, the `ServiceTrafficDistribution` feature is being used on.

### How `TopologyAwareHints` works

The [kube-proxy](#kube-proxy) section reveals the implementation details (and the drawbacks) of the `TopologyAwareHints` feature. For more details, see [upstream documentation](https://kubernetes.io/docs/concepts/services-networking/topology-aware-routing/) of the feature.

##### kube-proxy

By default, with kube-proxy running in `iptables` mode, traffic is distributed randomly across all endpoints, regardless of where it originates from. In a cluster with 3 zones, traffic is more likely to go to another zone than to stay in the current zone.
With the topology-aware routing feature, kube-proxy filters the endpoints it routes to based on the hints in the EndpointSlice resource. In most of the cases, kube-proxy will prefer the endpoint(s) in the same zone. For more details, see the [Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/#implementation-kube-proxy).

### How `ServiceTrafficDistribution` works

We reported the drawbacks related to the `TopologyAwareHints` feature in [kubernetes/kubernetes#113731](https://github.com/kubernetes/kubernetes/issues/113731). As result, the Kubernetes community implemented the `ServiceTrafficDistribution` feature.

The `ServiceTrafficDistribution` allows expressing preferences for how traffic should be routed to Service endpoints. For more details, see [upstream documentation](https://kubernetes.io/docs/reference/networking/virtual-ips/#traffic-distribution) of the feature.

The `PreferSameZone` strategy allows traffic to be routed to Service endpoints in topology-aware and predictable manner.

## How to make a Service topology-aware

### How to make a Service topology-aware in Kubernetes 1.32 to 1.33

In Kubernetes 1.32 to 1.33, `ServiceTrafficDistribution` is being used to make a Service topology-aware. The `.spec.trafficDistribution` field has to be set to `PreferClose`:

```yaml
apiVersion: v1
kind: Service
spec:
  trafficDistribution: PreferClose
```

### How to make a Service topology-aware in Kubernetes 1.34 and later

The value `PreferClose` has been deprecated in favor of `PreferSameZone` and `PreferSameNode`. `PreferSameZone` is an alias for the existing `PreferClose` to clarify its semantics. For more information, read the details in the [Kubernetes deprecation announcement](https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/#preferclose-traffic-distribution-is-deprecated).
In Kubernetes 1.34 and later, `ServiceTrafficDistribution` is still used to make a Service topology-aware. The `.spec.trafficDistribution` field should be set to `PreferSameZone`:

```yaml
apiVersion: v1
kind: Service
spec:
  trafficDistribution: PreferSameZone
```

## Prerequisites for making a Service topology-aware

1. The Pods backing the Service should be spread on most of the available zones. This constraint should be ensured with appropriate scheduling constraints (topology spread constraints, (anti-)affinity). Enabling the feature for a Service with a single backing Pod or Pods all located in the same zone does not lead to a benefit.
1. The component should be scaled up by `VerticalPodAutoscaler`. In case of an overload (a large portion of the of the traffic is originating from a given zone), the `VerticalPodAutoscaler` should provide better resource recommendations for the overloaded backing Pods.
1. Consider the [`TopologyAwareHints` constraints](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/#constraints).

> Note: The topology-aware routing feature is considered as alpha feature. Use it only for evaluation purposes.

## Topology-aware Services in the Seed cluster

##### etcd-main-client and etcd-events-client

The `etcd-main-client` and `etcd-events-client` Services are topology-aware. They are consumed by the kube-apiserver.

##### kube-apiserver

The `kube-apiserver` Service is topology-aware if the shoot uses layer 4 load-balancing. If it is using layer 7 load-balancing it is not. It is consumed by the controllers running in the Shoot control plane.  
Layer 7 load-balancing is active when `IstioTLSTermination` feature gate is active on the Seed and the Shoot did not opt out. Please see this [documentation](./kube_apiserver_loadbalancing.md) for more details.

> Note: The `istio-ingressgateway` component routes traffic in topology-aware manner - if possible, it routes traffic to the target `kube-apiserver` Pods in the same zone. If there is no healthy `kube-apiserver` Pod available in the same zone, the traffic is routed to any of the healthy Pods in the other zones. This behaviour is unconditionally enabled.

##### gardener-resource-manager

The `gardener-resource-manager` Service that is part of the Shoot control plane is topology-aware. The resource-manager serves webhooks and the Service is consumed by the kube-apiserver for the webhook communication.

##### vpa-webhook

The `vpa-webhook` Service that is part of the Shoot control plane is topology-aware. It is consumed by the kube-apiserver for the webhook communication.

## Topology-aware Services in the garden runtime cluster

##### virtual-garden-etcd-main-client and virtual-garden-etcd-events-client

The `virtual-garden-etcd-main-client` and `virtual-garden-etcd-events-client` Services are topology-aware. `virtual-garden-etcd-main-client` is consumed by `virtual-garden-kube-apiserver` and `gardener-apiserver`, `virtual-garden-etcd-events-client` is consumed by `virtual-garden-kube-apiserver`.

##### virtual-garden-kube-apiserver

The `virtual-garden-kube-apiserver` Service is topology-aware if it uses layer 4 load-balancing. If it is using layer 7 load-balancing it is not. It is consumed by `virtual-garden-kube-controller-manager`, `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`, extension admission components, `gardener-dashboard` and other components.
Layer 7 load-balancing is active when `IstioTLSTermination` feature gate is active in `gardener-operator`. Please see this [documentation](./kube_apiserver_loadbalancing.md) for more details.

##### gardener-apiserver

The `gardener-apiserver` Service is topology-aware. It is consumed by `virtual-garden-kube-apiserver`. The aggregation layer in `virtual-garden-kube-apiserver` proxies requests sent for the Gardener API types to the `gardener-apiserver`.

##### gardener-admission-controller

The `gardener-admission-controller` Service is topology-aware. It is consumed by `virtual-garden-kube-apiserver` and `gardener-apiserver` for the webhook communication.

## How to enable the topology-aware routing for a Seed cluster?

For a Seed cluster the topology-aware routing functionality can be enabled in the Seed specification:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
# ...
spec:
  settings:
    topologyAwareRouting:
      enabled: true
```

The topology-aware routing setting can be only enabled for a Seed cluster with more than one zone.
gardenlet enables topology-aware Services only for Shoot control planes with failure tolerance type `zone` (`.spec.controlPlane.highAvailability.failureTolerance.type=zone`). Control plane Pods of non-HA Shoots and HA Shoots with failure tolerance type `node` are pinned to single zone. For more details, see [High Availability Of Deployed Components](../development/high-availability-of-components.md).

## How to enable the topology-aware routing for a garden runtime cluster?

For a garden runtime cluster the topology-aware routing functionality can be enabled in the Garden resource specification:

```yaml
apiVersion: operator.gardener.cloud/v1alpha1
kind: Garden
# ...
spec:
  runtimeCluster:
    settings:
      topologyAwareRouting:
        enabled: true
```

The topology-aware routing setting can be only enabled for a garden runtime cluster with more than one zone.
