---
title: ExposureClasses
weight: 6
---

# ExposureClasses

The Gardener API server provides a cluster-scoped `ExposureClass` resource.
This resource is used to allow exposing the control plane of a Shoot cluster in various network environments like restricted corporate networks, DMZ, etc.

## Background

The `ExposureClass` resource is based on the concept for the `RuntimeClass` resource in Kubernetes.

A `RuntimeClass` abstracts the installation of a certain container runtime (e.g., gVisor, Kata Containers) on all nodes or a subset of the nodes in a Kubernetes cluster.
See [Runtime Class](https://kubernetes.io/docs/concepts/containers/runtime-class/) for more information.

In contrast, an `ExposureClass` abstracts the ability to expose a Shoot clusters control plane in certain network environments (e.g., corporate networks, DMZ, internet) on all Seeds or a subset of the Seeds.

Example: `RuntimeClass` and `ExposureClass`

```yaml
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: gvisor
handler: gvisorconfig
# scheduling:
#   nodeSelector:
#     env: prod
---
kind: ExposureClass
metadata:
  name: internet
handler: internet-config
# scheduling:
#   seedSelector:
#     matchLabels:
#       network/env: internet
```

Similar to `RuntimeClasses`, `ExposureClasses` also define a `.handler` field reflecting the name reference for the corresponding CRI configuration of the `RuntimeClass` and the control plane exposure configuration for the `ExposureClass`.

The CRI handler for `RuntimeClasses` is usually installed by an administrator (e.g., via a `DaemonSet` which installs the corresponding container runtime on the nodes).
The control plane exposure configuration for `ExposureClasses` will be also provided by an administrator.
This exposure configuration is part of the gardenlet configuration, as this component is responsible to configure the control plane accordingly.
See the [gardenlet Configuration `ExposureClass` Handlers](#gardenlet-configuration-exposureclass-handlers) section for more information.

The `RuntimeClass` also supports the selection of a node subset (which have the respective controller runtime binaries installed) for pod scheduling via its `.scheduling` section.
The `ExposureClass` also supports the selection of a subset of available Seed clusters whose gardenlet is capable of applying the exposure configuration for the Shoot control plane accordingly via its `.scheduling` section.

## Usage by a `Shoot`

A `Shoot` can reference an `ExposureClass` via the `.spec.exposureClassName` field.

> :warning: When creating a `Shoot` resource, the Gardener scheduler will try to assign the `Shoot` to a `Seed` which will host its control plane.

The scheduling behaviour can be influenced via the `.spec.seedSelectors` and/or `.spec.tolerations` fields in the `Shoot`.
`ExposureClass`es can also contain scheduling instructions.
If a `Shoot` is referencing an `ExposureClass`, then the scheduling instructions of both will be merged into the `Shoot`.
Those unions of scheduling instructions might lead to a selection of a `Seed` which is not able to deal with the `handler` of the `ExposureClass` and the `Shoot` creation might end up in an error.
In such case, the `Shoot` scheduling instructions should be revisited to check that they are not interfering with the ones from the `ExposureClass`.
If this is not feasible, then the combination with the `ExposureClass` might not be possible and you need to contact your Gardener administrator.

<details>
<summary>Example: Shoot and ExposureClass scheduling instructions merge flow</summary>

1. Assuming there is the following `Shoot` which is referencing the `ExposureClass` below:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: abc
  namespace: garden-dev
spec:
  exposureClassName: abc
  seedSelectors:
    matchLabels:
      env: prod
---
apiVersion: core.gardener.cloud/v1beta1
kind: ExposureClass
metadata:
  name: abc
handler: abc
scheduling:
  seedSelector:
    matchLabels:
      network: internal
```

2. Both `seedSelectors` would be merged into the `Shoot`. The result would be the following:
```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: abc
  namespace: garden-dev
spec:
  exposureClassName: abc
  seedSelectors:
    matchLabels:
      env: prod
      network: internal
```

3. Now the Gardener Scheduler would try to find a `Seed` with those labels.
  - If there are **no** Seeds with matching labels for the seed selector, then the `Shoot` will be unschedulable.
  - If there are Seeds with matching labels for the seed selector, then the Shoot will be assigned to the best candidate after the scheduling strategy is applied, see [Gardener Scheduler](../concepts/scheduler.md#algorithm-overview).
    - If the `Seed` is **not** able to serve the `ExposureClass` handler `abc`, then the Shoot will end up in error state.
    - If the `Seed` is able to serve the `ExposureClass` handler `abc`, then the `Shoot` will be created.

</details>

## gardenlet Configuration `ExposureClass` Handlers

The gardenlet is responsible to realize the control plane exposure strategy defined in the referenced `ExposureClass` of a `Shoot`.

Therefore, the `GardenletConfiguration` can contain an `.exposureClassHandlers` list with the respective configuration.

Example of the `GardenletConfiguration`:

```yaml
exposureClassHandlers:
- name: internet-config
  loadBalancerService:
    annotations:
      loadbalancer/network: internet
- name: internal-config
  loadBalancerService:
    annotations:
      loadbalancer/network: internal
  sni:
    ingress:
      namespace: ingress-internal
      labels:
        network: internal
```

Each gardenlet can define how the handler of a certain `ExposureClass` needs to be implemented for the Seed(s) where it is responsible for.

The `.name` is the name of the handler config and it must match to the `.handler` in the `ExposureClass`.

All control planes on a `Seed` are exposed via a load balancer, either a dedicated one or a central shared one.
The load balancer service needs to be configured in a way that it is reachable from the target network environment.
Therefore, the configuration of load balancer service need to be specified, which can be done via the `.loadBalancerService` section.
The common way to influence load balancer service behaviour is via annotations where the respective cloud-controller-manager will react on and configure the infrastructure load balancer accordingly.

The control planes on a `Seed` will be exposed via a central load balancer and with Envoy via TLS SNI passthrough proxy.
In this case, the gardenlet will install a dedicated ingress gateway (Envoy + load balancer + respective configuration) for each handler on the `Seed`.
The configuration of the ingress gateways can be controlled via the `.sni` section in the same way like for the default ingress gateways.
