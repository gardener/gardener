---
description: The basics of horizontal Node and vertical Pod auto-scaling
---

# Auto-Scaling in Shoot Clusters

There are three auto-scaling scenarios of relevance in Kubernetes clusters in general and Gardener shoot clusters in particular:

* Horizontal node auto-scaling, i.e., dynamically adding and removing worker nodes.
* Horizontal pod auto-scaling, i.e., dynamically adding and removing pod replicas.
* Vertical pod auto-scaling, i.e., dynamically raising or shrinking the resource requests/limits of pods.

This document provides an overview of these scenarios and how the respective auto-scaling components can be enabled and configured. For more details, please see our [pod auto-scaling best practices](shoot_pod_autoscaling_best_practices.md).

## Horizontal Node Auto-Scaling

Every shoot cluster that has at least one worker pool with `minimum < maximum` nodes configuration will get a `cluster-autoscaler` deployment.
Gardener is leveraging the upstream community Kubernetes [`cluster-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler).
We have forked it to [gardener/autoscaler](https://github.com/gardener/autoscaler/) so that it supports the way how Gardener manages the worker nodes (leveraging [gardener/machine-controller-manager](https://github.com/gardener/machine-controller-manager)).
However, we have not touched the logic how it performs auto-scaling decisions.
Consequently, please refer to the [official documentation](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler#faqdocumentation) for this component.

The `Shoot` API allows to configure a few flags of the `cluster-autoscaler`:

There are [general options for `cluster-autoscaler`](../../api-reference/core.md#core.gardener.cloud/v1beta1.ClusterAutoscaler), and these values will be used for all worker groups except for those overwriting them. Additionally, there are some [`cluster-autoscaler` flags to be set per worker pool](../../api-reference/core.md#core.gardener.cloud/v1beta1.ClusterAutoscalerOptions). They override any general value such as those specified in the general flags above.
> Only some `cluster-autoscaler` flags can be configured per worker pool, and is limited by NodeGroupAutoscalingOptions of the upstream community Kubernetes repository. This list can be found [here](https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/config/autoscaling_options.go#L37-L55).

## Horizontal Pod Auto-Scaling

This functionality (HPA) is a standard functionality of any Kubernetes cluster (implemented as part of the `kube-controller-manager` that all Kubernetes clusters have). It is always enabled.

The `Shoot` API allows to configure most of the [flags of the `horizontal-pod-autoscaler`](../../api-reference/core.md#core.gardener.cloud/v1beta1.HorizontalPodAutoscalerConfig).

## Vertical Pod Auto-Scaling

This form of auto-scaling (VPA) is enabled by default, but it can be switched off in the `Shoot` by setting `.spec.kubernetes.verticalPodAutoscaler.enabled=false` in case you deploy your own VPA into your cluster (having more than one VPA on the same set of pods will lead to issues, eventually).

Gardener is leveraging the upstream community Kubernetes [`vertical-pod-autoscaler`](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
If enabled, Gardener will deploy it as part of the control plane into the seed cluster.
It will also be used for the vertical autoscaling of Gardener's system components deployed into the `kube-system` namespace of shoot clusters, for example, `kube-proxy` or `metrics-server`.

You might want to refer to the [official documentation](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md) for this component to get more information how to use it.

The `Shoot` API allows to configure a few [flags of the `vertical-pod-autoscaler`](../../api-reference/core.md#core.gardener.cloud/v1beta1.VerticalPodAutoscaler).

⚠️ Please note that if you disable VPA, the related `CustomResourceDefinition`s (ours and yours) will remain in your shoot cluster (whether someone acts on them or not).
You can delete these `CustomResourceDefinition`s yourself using `kubectl delete crd` if you want to get rid of them (in case you statically size all resources, which we do not recommend).

# Pod Auto-Scaling Best Practices

Please continue reading our [pod auto-scaling best practices](shoot_pod_autoscaling_best_practices.md) for more details and recommendations.