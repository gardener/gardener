# Topology-Aware Traffic Routing

## Motivation

The enablement of [highly available shoot control-planes](../usage/high-availability/shoot_high_availability.md) requires multi-zone seed clusters. A garden runtime cluster can also be a multi-zone cluster. The topology-aware routing is introduced to reduce costs and to improve network performance by avoiding the cross availability zone traffic, if possible. The cross availability zone traffic is charged by the cloud providers and it comes with higher latency compared to the traffic within the same zone. The topology-aware routing feature enables topology-aware routing for `Service`s deployed in a seed or garden runtime cluster. For the clients consuming these topology-aware services, `kube-proxy` favors the endpoints which are located in the same zone where the traffic originated from. In this way, the cross availability zone traffic is avoided.

## How it works

The topology-aware routing feature relies on the Kubernetes features [`TopologyAwareHints`](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/) or [`ServiceTrafficDistribution`](https://kubernetes.io/docs/reference/networking/virtual-ips/#traffic-distribution) based on the runtime cluster's Kubernetes versions.

For Kubernetes versions < 1.31, the `TopologyAwareHints` feature is being used on in combination with the [EndpointSlice hints mutating webhook](#endpointslice-hints-mutating-webhook).

For Kubernetes versions >= 1.31, the `ServiceTrafficDistribution` feature is being used on. The [EndpointSlice hints mutating webhook](#endpointslice-hints-mutating-webhook) is enabled for Kubernetes 1.31 to allow graceful migration from `TopologyAwareHints` to `ServiceTrafficDistribution`.

### How `TopologyAwareHints` works

The [EndpointSlice hints mutating webhook](#endpointslice-hints-mutating-webhook) and [kube-proxy](#kube-proxy) sections reveal the implementation details (and the drawbacks) of the `TopologyAwareHints` feature. For more details, see [upstream documentation](https://kubernetes.io/docs/concepts/services-networking/topology-aware-routing/) of the feature.

##### EndpointSlice Hints Mutating Webhook

The component that is responsible for providing hints in the EndpointSlices resources is the [kube-controller-manager](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/), in particular this is the [EndpointSlice controller](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/). However, there are several drawbacks with the TopologyAwareHints feature that don't allow us to use it in its native way:

- The algorithm in the EndpointSlice controller is based on a CPU-balance heuristic. From the TopologyAwareHints documentation:
   > The controller allocates a proportional amount of endpoints to each zone. This proportion is based on the allocatable CPU cores for nodes running in that zone. For example, if one zone had 2 CPU cores and another zone only had 1 CPU core, the controller would allocate twice as many endpoints to the zone with 2 CPU cores.

   In case it is not possible to achieve a balanced distribution of the endpoints, as a safeguard mechanism the controller removes hints from the EndpointSlice resource.
   In our setup, the clients and the servers are well-known and usually the traffic a component receives does not depend on the zone's allocatable CPU.
   Many components deployed by Gardener are scaled automatically by VPA. In case of an overload of a replica, the VPA should provide and apply enhanced CPU and memory resources. Additionally, Gardener uses the cluster-autoscaler to upscale/downscale Nodes dynamically. Hence, it is not possible to ensure a balanced allocatable CPU across the zones.
- The TopologyAwareHints feature does not work at low-endpoint counts. It falls apart for a Service with less than 10 Endpoints.
- Hints provided by the EndpointSlice controller are not deterministic. With cluster-autoscaler running and load increasing, hints can be removed in the next moment. There is no option to enforce the zone-level topology.

For more details, see the following issue [kubernetes/kubernetes#113731](https://github.com/kubernetes/kubernetes/issues/113731).

To circumvent these issues with the EndpointSlice controller, a mutating webhook in the gardener-resource-manager assigns hints to EndpointSlice resources. For each endpoint in the EndpointSlice, it sets the endpoint's hints to the endpoint's zone. The webhook overwrites the hints provided by the EndpointSlice controller in kube-controller-manager. For more details, see the [webhook's documentation](../concepts/resource-manager.md#endpointslice-hints).

##### kube-proxy

By default, with kube-proxy running in `iptables` mode, traffic is distributed randomly across all endpoints, regardless of where it originates from. In a cluster with 3 zones, traffic is more likely to go to another zone than to stay in the current zone.
With the topology-aware routing feature, kube-proxy filters the endpoints it routes to based on the hints in the EndpointSlice resource. In most of the cases, kube-proxy will prefer the endpoint(s) in the same zone. For more details, see the [Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/#implementation-kube-proxy).

### How `ServiceTrafficDistribution` works

We reported the drawbacks related to the `TopologyAwareHints` feature in [kubernetes/kubernetes#113731](https://github.com/kubernetes/kubernetes/issues/113731). As result, the Kubernetes community implemented the `ServiceTrafficDistribution` feature.

The `ServiceTrafficDistribution` allows expressing preferences for how traffic should be routed to Service endpoints. For more details, see [upstream documentation](https://kubernetes.io/docs/reference/networking/virtual-ips/#traffic-distribution) of the feature.

The `PreferClose` strategy with kube-proxy of `ServiceTrafficDistribution` allows traffic to be routed to Service endpoints in topology-aware and predictable manner.
It is simpler than `service.kubernetes.io/topology-mode: auto` - if there are Service endpoints which reside in the same zone as the client, traffic is routed to one of the endpoints within the same zone as the client. If the client's zone does not have any available Service endpoints, traffic is routed to any available endpoint within the cluster.

## How to make a Service topology-aware

### How to make a Service topology-aware using `TopologyAwareHints` (Kubernetes < 1.31)

In Kubernetes < 1.31, to make a Service topology-aware the following annotation and label have to be added to the Service:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.kubernetes.io/topology-mode: "auto"
  labels:
    endpoint-slice-hints.resources.gardener.cloud/consider: "true"
```

The `service.kubernetes.io/topology-mode=auto` annotation is needed for kube-proxy. One of the prerequisites on kube-proxy side for using topology-aware routing is the corresponding Service to be annotated with the `service.kubernetes.io/topology-mode=auto`. For more details, see the following [kube-proxy function](https://github.com/kubernetes/kubernetes/blob/b46a3f887ca979b1a5d14fd39cb1af43e7e5d12d/pkg/proxy/topology.go#L140-L186).
The `endpoint-slice-hints.resources.gardener.cloud/consider=true` label is needed for gardener-resource-manager to prevent the EndpointSlice hints mutating webhook from selecting all EndpointSlice resources but only the ones that are labeled with the consider label.

The Gardener extensions can use this approach to make a Service they deploy topology-aware.

### How to make a Service topology-aware using `ServiceTrafficDistribution` (Kubernetes == 1.31)

In Kubernetes 1.31, to make a Service topology-aware the `.spec.trafficDistribution` field has to be set to `PreferClose` and the label `endpoint-slice-hints.resources.gardener.cloud/consider=true` needs to be added:

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    endpoint-slice-hints.resources.gardener.cloud/consider: "true"
spec:
  trafficDistribution: PreferClose
```

### How to make a Service topology-aware using `ServiceTrafficDistribution` (Kubernetes >= 1.31)

In Kubernetes >= 1.31, to make a Service topology-aware the `.spec.trafficDistribution` field has to be set to `PreferClose`:

```yaml
apiVersion: v1
kind: Service
spec:
  trafficDistribution: PreferClose
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

The `kube-apiserver` Service is topology-aware. It is consumed by the controllers running in the Shoot control plane.

> Note: The `istio-ingressgateway` component routes traffic in topology-aware manner - if possible, it routes traffic to the target `kube-apiserver` Pods in the same zone. If there is no healthy `kube-apiserver` Pod available in the same zone, the traffic is routed to any of the healthy Pods in the other zones. This behaviour is unconditionally enabled.

##### gardener-resource-manager

The `gardener-resource-manager` Service that is part of the Shoot control plane is topology-aware. The resource-manager serves webhooks and the Service is consumed by the kube-apiserver for the webhook communication.

##### vpa-webhook

The `vpa-webhook` Service that is part of the Shoot control plane is topology-aware. It is consumed by the kube-apiserver for the webhook communication.

## Topology-aware Services in the garden runtime cluster

##### virtual-garden-etcd-main-client and virtual-garden-etcd-events-client

The `virtual-garden-etcd-main-client` and `virtual-garden-etcd-events-client` Services are topology-aware. `virtual-garden-etcd-main-client` is consumed by `virtual-garden-kube-apiserver` and `gardener-apiserver`, `virtual-garden-etcd-events-client` is consumed by `virtual-garden-kube-apiserver`.

##### virtual-garden-kube-apiserver

The `virtual-garden-kube-apiserver` Service is topology-aware. It is consumed by `virtual-garden-kube-controller-manager`, `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller`, extension admission components, `gardener-dashboard` and other components.

> Note: Unlike the other Services, the `virtual-garden-kube-apiserver` Service is of type LoadBalancer. In-cluster components consuming the `virtual-garden-kube-apiserver` Service by its Service name will have benefit from the topology-aware routing. However, the TopologyAwareHints feature cannot help with external traffic routed to load balancer's address - such traffic won't be routed in a topology-aware manner and will be routed according to the cloud-provider specific implementation.

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
