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
Consequently, please refer to the [offical documentation](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler#faqdocumentation) for this component.

The `Shoot` API allows to configure a few flags of the `cluster-autoscaler`:
* `.spec.kubernetes.clusterAutoscaler.scaleDownDelayAfterAdd` defines how long after scale up that scale down evaluation resumes (default: `1h`).
* `.spec.kubernetes.clusterAutoscaler.scaleDownDelayAfterDelete` defines how long after node deletion that scale down evaluation resumes (defaults to `ScanInterval`).
* `.spec.kubernetes.clusterAutoscaler.scaleDownDelayAfterFailure` defines how long after scale down failure that scale down evaluation resumes (default: `3m`).
* `.spec.kubernetes.clusterAutoscaler.scaleDownUnneededTime` defines how long a node should be unneeded before it is eligible for scale down (default: `30m`).
* `.spec.kubernetes.clusterAutoscaler.scaleDownUtilizationThreshold` defines the threshold under which a node is being removed (default: `0.5`).
* `.spec.kubernetes.clusterAutoscaler.scanInterval` defines how often cluster is reevaluated for scale up or down (default: `10s`). 
* `.spec.kubernetes.clusterAutoscaler.ignoreTaints` specifies a list of taint keys to ignore in node templates when considering to scale a node group (default: `nil`). 

## Vertical Pod Auto-Scaling

This form of auto-scaling is not enabled by default and must be explicitly enabled in the `Shoot` by setting `.spec.kubernetes.verticalPodAutoscaler.enabled=true`.
The reason is that it was only introduced lately, and some end-users might have already deployed their own VPA into their clusters, i.e., enabling it by default would interfere with such custom deployments and lead to issues, eventually.

Gardener is also leveraging an upstream community tool, i.e., the Kubernetes [`vertical-pod-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
If enabled, Gardener will deploy it as part of the control plane into the seed cluster.
It will also be used for the vertical autoscaling of Gardener's system components deployed into the `kube-system` namespace of shoot clusters, for example, `kube-proxy` or `metrics-server`.

You might want to refer to the [official documentation](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md) for this component to get more information how to use it.

The `Shoot` API allows to configure a few flags of the `vertical-pod-autoscaler`:

* `.spec.kubernetes.verticalPodAutoscaler.evictAfterOOMThreshold` defines the threshold that will lead to pod eviction in case it OOMed in less than the given threshold since its start and if it has only one container (default: `10m0s`).
* `.spec.kubernetes.verticalPodAutoscaler.evictionRateBurst` defines the burst of pods that can be evicted (default: `1`).
* `.spec.kubernetes.verticalPodAutoscaler.evictionRateLimit` defines the number of pods that can be evicted per second. A rate limit set to 0 or -1 will disable the rate limiter (default: `-1`).
* `.spec.kubernetes.verticalPodAutoscaler.evictionTolerance` defines the fraction of replica count that can be evicted for update in case more than one pod can be evicted (default: `0.5`).
* `.spec.kubernetes.verticalPodAutoscaler.recommendationMarginFraction` is the fraction of usage added as the safety margin to the recommended request (default: `0.15`).
* `.spec.kubernetes.verticalPodAutoscaler.updaterInterval` is the interval how often the updater should run (default: `1m0s`).
* `.spec.kubernetes.verticalPodAutoscaler.recommenderInterval` is the interval how often metrics should be fetched (default: `1m0s`).

⚠️ Please note that if you disable the VPA again, then the related `CustomResourceDefinition`s will remain in your shoot cluster (although, nobody will act on them).
This will also keep all existing `VerticalPodAutoscaler` objects in the system, including those that might be created by you. You can delete the `CustomResourceDefinition`s yourself using `kubectl delete crd` if you want to get rid of them.
