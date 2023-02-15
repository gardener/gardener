# Readiness of Shoot Worker Nodes

## Background

When registering new `Nodes`, kubelet adds the `node.kubernetes.io/not-ready` taint to prevent scheduling workload Pods to the `Node` until the `Ready` condition gets `True`.
However, the kubelet does not consider the readiness of node-critical Pods.
Hence, the `Ready` condition might get `True` and the `node.kubernetes.io/not-ready` taint might get removed, for example, before the CNI daemon Pod (e.g. `calico-node`) has successfully placed the CNI binaries on the machine.

This problem has been discussed extensively in kubernetes, e.g., in [kubernetes/kubernetes#75890](https://github.com/kubernetes/kubernetes/issues/75890).
However, several proposals have been rejected because the problem can be solved by using the `--register-with-taints` kubelet flag and dedicated controllers ([ref](https://github.com/kubernetes/enhancements/pull/1003#issuecomment-619087019)).

## Implementation in Gardener

Gardener makes sure that workload Pods are only scheduled to `Nodes` where all node-critical components required for running workload Pods are ready. 
For this, Gardener follows the proposed solution by the Kubernetes community and registers new `Node` objects with the `node.gardener.cloud/critical-components-not-ready` taint (effect `NoSchedule`).
gardener-resource-manager's [`Node` controller](../concepts/resource-manager.md#node-controller) reacts on newly created `Node` objects that have this taint.
The controller removes the taint once all node-critical Pods are ready (determined by checking the Pods' `Ready` conditions).

The `Node` controller considers all `DaemonSets` and `Pods` with the label `node.gardener.cloud/critical-component=true` as node-critical.
If there are `DaemonSets` that contain the `node.gardener.cloud/critical-component=true` label in their metadata and in their Pod template, the `Node` controller waits for corresponding daemon Pods to be scheduled and to get ready before removing the taint.

## Marking Node-Critical Components

To make use of this feature, node-critical DaemonSets and Pods need to

- tolerate the `node.gardener.cloud/critical-components-not-ready` `NoSchedule` taint and
- be labelled with `node.gardener.cloud/critical-component=true`.

Gardener already marks components like kube-proxy, apiserver-proxy and node-local-dns as node-critical.
Provider extensions mark components like csi-driver-node as node-critical.
Network extensions mark components responsible for setting up CNI on worker Nodes (e.g. `calico-node`) as node-critical.
If shoot owners manage any additional node-critical components, they can make use of this feature as well.
