---
weight: 9
description: The basics of Horizontal Node and Vertical Pod Auto-Scaling
---

# Auto-Scaling in Shoot Clusters

There are two parts that relate to auto-scaling in Kubernetes clusters in general:

* Horizontal node auto-scaling, i.e., dynamically adding and removing worker nodes.
* Vertical pod auto-scaling, i.e., dynamically raising or shrinking the resource requests/limits of pods.

This document provides an overview of both scenarios.

## Horizontal Node Auto-Scaling

Every shoot cluster that has at least one worker pool with `minimum < maximum` nodes configuration will get a `cluster-autoscaler` deployment.
Gardener is leveraging the upstream community Kubernetes [`cluster-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler).
We have forked it to [gardener/autoscaler](https://github.com/gardener/autoscaler/) so that it supports the way how Gardener manages the worker nodes (leveraging [gardener/machine-controller-manager](https://github.com/gardener/machine-controller-manager)).
However, we have not touched the logic how it performs auto-scaling decisions.
Consequently, please refer to the [official documentation](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler#faqdocumentation) for this component.

The `Shoot` API allows to configure a few flags of the `cluster-autoscaler`:

There are [general options for `cluster-autoscaler`](../../api-reference/core.md#core.gardener.cloud/v1beta1.ClusterAutoscaler), and these values will be used for all worker groups except for those overwriting them. Additionally, there are some [`cluster-autoscaler` flags to be set per worker pool](../../api-reference/core.md#core.gardener.cloud/v1beta1.ClusterAutoscalerOptions). They override any general value such as those specified in the general flags above.
> Only some `cluster-autoscaler` flags can be configured per worker pool, and is limited by NodeGroupAutoscalingOptions of the upstream community Kubernetes repository. This list can be found [here](https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/config/autoscaling_options.go#L37-L55).

## Vertical Pod Auto-Scaling

This form of auto-scaling is not enabled by default and must be explicitly enabled in the `Shoot` by setting `.spec.kubernetes.verticalPodAutoscaler.enabled=true`.
The reason is that it was only introduced lately, and some end-users might have already deployed their own VPA into their clusters, i.e., enabling it by default would interfere with such custom deployments and lead to issues, eventually. Also if `ShootVPAEnabledByDefault` admissionPlugins is enabled then `.spec.kubernetes.verticalPodAutoscaler.enabled` will be set to true.

Gardener is also leveraging an upstream community tool, i.e., the Kubernetes [`vertical-pod-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
If enabled, Gardener will deploy it as part of the control plane into the seed cluster.
It will also be used for the vertical autoscaling of Gardener's system components deployed into the `kube-system` namespace of shoot clusters, for example, `kube-proxy` or `metrics-server`.

You might want to refer to the [official documentation](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md) for this component to get more information how to use it.

The `Shoot` API allows to configure a few [flags of the `vertical-pod-autoscaler`](../../api-reference/core.md#core.gardener.cloud/v1beta1.VerticalPodAutoscaler).

⚠️ Please note that if you disable the VPA again, then the related `CustomResourceDefinition`s will remain in your shoot cluster (although, nobody will act on them).
This will also keep all existing `VerticalPodAutoscaler` objects in the system, including those that might be created by you. You can delete the `CustomResourceDefinition`s yourself using `kubectl delete crd` if you want to get rid of them.
