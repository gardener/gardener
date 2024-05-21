---
title: Marking Node-Critical Components and csi-driver-node Pods
weight: 1
---
# Marking Node-Critical Components and `csi-driver-node` Pods 

## Background

When registering new `Nodes`, kubelet adds the `node.kubernetes.io/not-ready` taint to prevent scheduling workload Pods to the `Node` until the `Ready` condition gets `True`.
However, the kubelet does not consider the readiness of node-critical Pods.
Hence, the `Ready` condition might get `True` and the `node.kubernetes.io/not-ready` taint might get removed, for example, before the CNI daemon Pod (e.g., `calico-node`) has successfully placed the CNI binaries on the machine.

This problem has been discussed extensively in kubernetes, e.g., in [kubernetes/kubernetes#75890](https://github.com/kubernetes/kubernetes/issues/75890).
However, several proposals have been rejected because the problem can be solved by using the `--register-with-taints` kubelet flag and dedicated controllers ([ref](https://github.com/kubernetes/enhancements/pull/1003#issuecomment-619087019)).

## Implementation in Gardener

Gardener makes sure that workload Pods are only scheduled to `Nodes` where all node-critical components required for running workload Pods are ready. 
For this, Gardener follows the proposed solution by the Kubernetes community and registers new `Node` objects with the `node.gardener.cloud/critical-components-not-ready` taint (effect `NoSchedule`).
gardener-resource-manager's [`Node` controller](../concepts/resource-manager.md#node-controller) reacts on newly created `Node` objects that have this taint.
The controller removes the taint once all node-critical Pods are ready (determined by checking the Pods' `Ready` conditions).

The `Node` controller considers all `DaemonSets` and `Pods` with the label `node.gardener.cloud/critical-component=true` as node-critical.
If there are `DaemonSets` that contain the `node.gardener.cloud/critical-component=true` label in their metadata and in their Pod template, the `Node` controller waits for corresponding daemon Pods to be scheduled and to get ready before removing the taint.

Additionally, the `Node` controller checks for the readiness of `csi-driver-node` components if a respective Pod indicates that it uses such a driver.
This is achieved through a well-defined annotation prefix (`node.gardener.cloud/wait-for-csi-node-`).
For example, the `csi-driver-node` Pod for Openstack Cinder is annotated with `node.gardener.cloud/wait-for-csi-node-cinder=cinder.csi.openstack.org`.
A key prefix is used instead of a "regular" annotation to allow for multiple CSI drivers being registered by one `csi-driver-node` Pod.
The annotation key's suffix can be chosen arbitrarily (in this case `cinder`) and the annotation value needs to match the actual driver name as specified in the `CSINode` object.
The `Node` controller will verify that the used driver is properly registered in this object before removing the `node.gardener.cloud/critical-components-not-ready` taint.
Note that the `csi-driver-node` Pod still needs to be labelled and tolerate the taint as described above to be considered in this additional check.

## Marking Node-Critical Components and `csi-driver-node` Pods

To make use of this feature, node-critical DaemonSets and Pods need to:

- Tolerate the `node.gardener.cloud/critical-components-not-ready` `NoSchedule` taint.
- Be labelled with `node.gardener.cloud/critical-component=true`.

`csi-driver-node` Pods additionally need to:

- Be annotated with `node.gardener.cloud/wait-for-csi-node-<name>=<full-driver-name>`.
  It's required that these Pods fulfill the above criteria (label and toleration) as well.

Gardener already marks components like kube-proxy, apiserver-proxy and node-local-dns as node-critical.
Provider extensions mark components like csi-driver-node as node-critical and add the `wait-for-csi-node` annotation.
Network extensions mark components responsible for setting up CNI on worker Nodes (e.g., `calico-node`) as node-critical.
If shoot owners manage any additional node-critical components, they can make use of this feature as well.
