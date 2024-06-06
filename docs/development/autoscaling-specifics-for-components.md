---
title: Autoscaling Specifics for Components
---

## Overview

This document describes the used autoscaling mechanism for several components.

## Garden or Shoot Cluster etcd

By default, if none of the autoscaling modes is requested the `etcd` is deployed with static resources, without autoscaling.

However, there are two supported autoscaling modes for the Garden or Shoot cluster etcd.

- `HVPA`

   In `HVPA` mode, the etcd is scaled by the [hvpa-controller](https://github.com/gardener/hvpa-controller). The gardenlet/gardener-operator is creating an `HVPA` resource for the etcd (`main` or `events`).
   The `HVPA` enables a vertical scaling for etcd.

   The `HVPA` mode is the used autoscaling mode when the `HVPA` feature gate is enabled and the `VPAForETCD` feature gate is disabled.

- `VPA`

   In `VPA` mode, the etcd is scaled by a native `VPA` resource.

   The `VPA` mode is the used autoscaling mode when the `VPAForETCD` feature gate is enabled (takes precedence over the `HVPA` feature gate). 

> [!NOTE]
> Starting with release `v1.97`, the `VPAForETCD` feature gate is enabled by default.

For both of the autoscaling modes downscaling is handled more pessimistically to prevent many subsequent etcd restarts. Thus, for `production` and `infrastructure` Shoot clusters (or all Garden clusters), downscaling is deactivated for the main etcd. For all other Shoot clusters, lower advertised requests/limits are only applied during the Shoot's maintenance time window.

## Shoot Kubernetes API Server

There are three supported autoscaling modes for the Shoot Kubernetes API server.

- `Baseline`

   In `Baseline` mode, the Shoot Kubernetes API server is scaled by active HPA and VPA in passive, recommend-only mode.

   The API server resource requests are computed based on the Shoot's minimum Nodes count:
   | Range       | Resource Requests |
   |-------------|-------------------|
   | [0, 2]      | `800m`, `800Mi`   |
   | (2, 10]     | `1000m`, `1100Mi` |
   | (10, 50]    | `1200m`, `1600Mi` |
   | (50, 100]   | `2500m`, `5200Mi` |
   | (100, inf.) | `3000m`, `5200Mi` |

   The `Baseline` mode is the used autoscaling mode when the `HVPA` and `VPAAndHPAForAPIServer` feature gates are not enabled.

- `HVPA`

   In `HVPA` mode, the Shoot Kubernetes API server is scaled by the [hvpa-controller](https://github.com/gardener/hvpa-controller). The gardenlet is creating an `HVPA` resource for the API server. The `HVPA` resource is backed by HPA and VPA both in recommend-only mode. The hvpa-controller is responsible for enabling simultaneous horizontal and vertical scaling by incorporating the recommendations from the HPA and VPA.

   The initial API server resource requests are `500m` and `1Gi`.
   HVPA's HPA is scaling only on CPU (average utilization 80%). HVPA's VPA max allowed values are `8` CPU and `25G`.

   The `HVPA` mode is the used autoscaling mode when the `HVPA` feature gate is enabled (and the `VPAAndHPAForAPIServer` feature gate is disabled).

- `VPAAndHPA`

   In `VPAAndHPA` mode, the Shoot Kubernetes API server is scaled simultaneously by VPA and HPA on the same metric (CPU and memory usage). The pod-trashing cycle between VPA and HPA scaling on the same metric is avoided by configuring the HPA to scale on average usage (not on average utilization) and by picking the target average utilization values in sync with VPA's allowed maximums. This makes possible VPA to first scale vertically on CPU/memory usage. Once all Pods' average CPU/memory usage is close to exceed the VPA's allowed maximum CPU/memory (the HPA's target average utilization, 1/7 less than VPA's allowed maximums), HPA is scaling horizontally (by adding a new replica).

   The `VPAAndHPA` mode is introduced to address disadvantages with HVPA: additional component; modifies the deployment triggering unnecessary rollouts; vertical scaling only at max replicas; stuck vertical resource requests when scaling in again; etc.

   The initial API server resource requests are `250m` and `500Mi`.
   VPA's max allowed values are `7` CPU and `28G`. HPA's average target usage values are `6` CPU and `24G`.

   The `VPAAndHPA` mode is the used autoscaling mode when the `VPAAndHPAForAPIServer` feature gate is enabled (takes precedence over the `HVPA` feature gate).

The API server's replica count in all scaling modes varies between 2 and 3. The min replicas count of 2 is imposed by the [High Availability of Shoot Control Plane Components](../development/high-availability.md#control-plane-components).

The gardenlet sets the initial API server resource requests only when the Deployment is not found. When the Deployment exists, it is not overwriting the kube-apiserver container resources.

## Disabling Scale Down for Components in the Shoot Control Plane

Some Shoot clusters' control plane components can be overloaded and can have very high resource usage. The existing autoscaling solution could be imperfect to cover these cases. Scale down actions for such overloaded components could be disruptive.

To prevent such disruptive scale-down actions it is possible to disable scale down of the etcd, Kubernetes API server and Kubernetes controller manager in the Shoot control plane by annotating the Shoot with `alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled=true`.

There are the following specifics for when disabling scale-down for the Kubernetes API server component:
- In `Baseline` and `HVPA` modes the HPA's min and max replicas count are set to 4.
- In `VPAAndHPA` mode if the HPA resource exists and HPA's `spec.minReplicas` is not nil then the min replicas count is `max(spec.minReplicas, status.desiredReplicas)`. When scale-down is disabled, this allows operators to specify a custom value for HPA `spec.minReplicas` and this value not to be reverted by gardenlet. I.e, HPA _does_ scale down to min replicas but not below min replicas. HPA's max replicas count is 4.

> Note: The `alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled` annotation is alpha and can be removed anytime without further notice. Only use it if you know what you do.

##  Virtual Kubernetes API Server and Gardener API Server

The virtual Kubernetes API server's autoscaling is same as the Shoot Kubernetes API server's with the following differences:
- The initial API server resource requests are `600m` and `512Mi` in all autoscaling modes.
- The min replicas count is 2 for a non-HA virtual cluster and 3 for an HA virtual cluster. The max replicas count is 6.
- In `HVPA` mode, HVPA's HPA is scaling on both CPU and memory (average utilization 80% for both).

The Gardener API server's autoscaling is the same as the Shoot Kubernetes API server's with the following differences:
- The initial API server resource requests are `600m` and `512Mi` in all autoscaling modes.
- The min replicas count is 2. The max replicas count is 4.
- In `HVPA` mode, HVPA's HPA is scaling on both CPU and memory (average utilization 80% for both).
- In `HVPA` mode, HVPA's VPA max allowed values are `4` CPU and `25G`.
