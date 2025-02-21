# `PriorityClass`es in Gardener Clusters

Gardener makes use of [`PriorityClass`es](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) to improve the overall robustness of the system.
In order to benefit from the full potential of `PriorityClass`es, the gardenlet manages a set of well-known `PriorityClass`es with fine-granular priority values.

All components of the system should use these well-known `PriorityClass`es instead of creating and using separate ones with arbitrary values, which would compromise the overall goal of using `PriorityClass`es in the first place.
The gardenlet manages the well-known `PriorityClass`es listed in this document, so that third parties (e.g., Gardener extensions) can rely on them to be present when deploying components to Seed and Shoot clusters.

The listed well-known `PriorityClass`es follow this rough concept:

- Values are close to the maximum that can be declared by the user. This is important to ensure that Shoot system components have higher priority than the workload deployed by end-users.
- Values have a bit of headroom in between to ensure flexibility when the need for intermediate priority values arises.
- Values of `PriorityClass`es created on Seed clusters are lower than the ones on Shoots to ensure that Shoot system components have higher priority than Seed components, if the Seed is backed by a Shoot (`ManagedSeed`), e.g. `coredns` should have higher priority than `gardenlet`.
- Names simply include the last digits of the value to minimize confusion caused by many (similar) names like `critical`, `importance-high`, etc.

## Garden Clusters

When using the `gardener-operator` for managing the garden runtime and virtual cluster, the following `PriorityClass`es are available:

### `PriorityClass`es for Garden Control Plane Components

| Name                              | Priority  | Associated Components (Examples)                                                                                                                                                                                     |
|-----------------------------------|-----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gardener-garden-system-critical` | 999999550 | `gardener-operator`, `gardener-resource-manager`, `istio`                                                                                                                                                            |
| `gardener-garden-system-500`      | 999999500 | `virtual-garden-etcd-events`, `virtual-garden-etcd-main`, `virtual-garden-kube-apiserver`, `gardener-apiserver`                                                                                                      |
| `gardener-garden-system-400`      | 999999400 | `virtual-garden-gardener-resource-manager`, `gardener-admission-controller`, Extension Admission Controllers                                                                                                         |
| `gardener-garden-system-300`      | 999999300 | `virtual-garden-kube-controller-manager`, `vpa-admission-controller`, `etcd-druid`, `nginx-ingress-controller`                                                                                                       |
| `gardener-garden-system-200`      | 999999200 | `vpa-recommender`, `vpa-updater`, `gardener-scheduler`, `gardener-controller-manager`, `gardener-dashboard`, `terminal-controller-manager`, `gardener-discovery-server`, Extension Controllers                       |
| `gardener-garden-system-100`      | 999999100 | `fluent-operator`, `fluent-bit`, `gardener-metrics-exporter`, `kube-state-metrics`, `plutono`, `vali`, `prometheus-operator`, `alertmanager-garden`, `prometheus-garden`, `blackbox-exporter`, `prometheus-longterm` |

## Seed Clusters

### `PriorityClass`es for Seed System Components

| Name                               | Priority  | Associated Components (Examples)                                                                                                                                                                                                     |
|------------------------------------|-----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gardener-system-critical`         | 999998950 | `gardenlet`, `gardener-resource-manager`, `istio-ingressgateway`, `istiod`                                                                                                                                                           |
| `gardener-system-900`              | 999998900 | Extensions                                                                                                                                                                                               |
| `gardener-system-800`              | 999998800 | `dependency-watchdog-endpoint`, `dependency-watchdog-probe`, `etcd-druid`, `vpa-admission-controller`                                                                                                                                |
| `gardener-system-700`              | 999998700 | `vpa-recommender`, `vpa-updater`                                                                                                                                                                                                     |
| `gardener-system-600`              | 999998600 | `alertmanager-seed`, `fluent-operator`, `fluent-bit`, `plutono`, `kube-state-metrics`, `nginx-ingress-controller`, `nginx-k8s-backend`, `prometheus-operator`, `prometheus-aggregate`, `prometheus-cache`, `prometheus-seed`, `vali` |
| `gardener-reserve-excess-capacity` | -5        | `reserve-excess-capacity` ([ref](https://github.com/gardener/gardener/pull/6135))                                                                                                                                                    |

### `PriorityClass`es for Shoot Control Plane Components

| Name                  | Priority  | Associated Components (Examples)                                                                                                                                                       |
|-----------------------|-----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gardener-system-500` | 999998500 | `etcd-events`, `etcd-main`, `kube-apiserver`                                                                                                                                           |
| `gardener-system-400` | 999998400 | `gardener-resource-manager`                                                                                                                                                            |
| `gardener-system-300` | 999998300 | `cloud-controller-manager`, `cluster-autoscaler`, `csi-driver-controller`, `kube-controller-manager`, `kube-scheduler`, `machine-controller-manager`, `terraformer`, `vpn-seed-server` |
| `gardener-system-200` | 999998200 | `csi-snapshot-controller`, `csi-snapshot-validation`, `cert-controller-manager`, `shoot-dns-service`, `vpa-admission-controller`, `vpa-recommender`, `vpa-updater`                     |
| `gardener-system-100` | 999998100 | `alertmanager-shoot`, `plutono`, `kube-state-metrics`, `prometheus-shoot`, `blackbox-exporter`, `vali`, `event-logger`                                                                 |

## Shoot Clusters

## `PriorityClass`es for Shoot System Components

| Name                                              | Priority   | Associated Components (Examples)                                                                                            |
|---------------------------------------------------|------------|-----------------------------------------------------------------------------------------------------------------------------|
| `system-node-critical` (created by Kubernetes)    | 2000001000 | `calico-node`, `kube-proxy`, `apiserver-proxy`, `csi-driver`, `egress-filter-applier`                                       |
| `system-cluster-critical` (created by Kubernetes) | 2000000000 | `calico-typha`, `calico-kube-controllers`, `coredns`, `vpn-shoot`, `registry-cache`                                         |
| `gardener-shoot-system-900`                       | 999999900  | `node-problem-detector`                                                                                                     |
| `gardener-shoot-system-800`                       | 999999800  | `calico-typha-horizontal-autoscaler`, `calico-typha-vertical-autoscaler`                                                    |
| `gardener-shoot-system-700`                       | 999999700  | `blackbox-exporter`, `node-exporter`                                                                                        |
| `gardener-shoot-system-600`                       | 999999600  | `addons-nginx-ingress-controller`, `addons-nginx-ingress-k8s-backend`, `kubernetes-dashboard`, `kubernetes-metrics-scraper` |
