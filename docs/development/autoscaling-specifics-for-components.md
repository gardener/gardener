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

> [!NOTE]
> Starting with release `v1.106`, the `HVPA` feature gate is deprecated and locked to false.

- `VPA`

   In `VPA` mode, the etcd is scaled by a native `VPA` resource.

   The `VPA` mode is the used autoscaling mode when the `VPAForETCD` feature gate is enabled (takes precedence over the `HVPA` feature gate).

> [!NOTE]
> Starting with release `v1.97`, the `VPAForETCD` feature gate is enabled by default.
> Starting with release `v1.105`, the `VPAForETCD` feature gate is promoted to GA and locked to true.

For both of the autoscaling modes downscaling is handled more pessimistically to prevent many subsequent etcd restarts. Thus, for `production` and `infrastructure` Shoot clusters (or all Garden clusters), downscaling is deactivated for the main etcd. For all other Shoot clusters, lower advertised requests/limits are only applied during the Shoot's maintenance time window.

## Shoot Kubernetes API Server

The Shoot Kubernetes API server is scaled simultaneously by VPA and HPA on the same metric (CPU and memory usage).

The pod-trashing cycle between VPA and HPA scaling on the same metric is avoided by configuring the HPA to scale on average usage (not on average utilization).
This makes possible VPA to first scale vertically on CPU/memory usage.
Once all Pods' average CPU/memory usage exceeds the HPA's target average usage, HPA is scaling horizontally (by adding a new replica).HPA's average target usage values are `6` CPU and `24G`.
The initial API server resource requests are `250m` and `500Mi`.

The API server's min replicas count is 2, the max replicas count - 6.
The min replicas count of 2 is imposed by the [High Availability of Shoot Control Plane Components](../development/high-availability-of-components.md#control-plane-components).

The gardenlet sets the initial API server resource requests only when the Deployment is not found. When the Deployment exists, it is not overwriting the kube-apiserver container resources.

## Disabling Scale Down for Components in the Shoot Control Plane

Some Shoot clusters' control plane components can be overloaded and can have very high resource usage. The existing autoscaling solution could be imperfect to cover these cases. Scale down actions for such overloaded components could be disruptive.

To prevent such disruptive scale-down actions it is possible to disable scale down of the etcd, Kubernetes API server and Kubernetes controller manager in the Shoot control plane by annotating the Shoot with `alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled=true`.

There is the following specific for when disabling scale-down for the Kubernetes API server component:
- If the HPA resource exists and HPA's `spec.minReplicas` is not nil then the min replicas count is `max(spec.minReplicas, status.desiredReplicas)`. When scale-down is disabled, this allows operators to specify a custom value for HPA `spec.minReplicas` and this value not to be reverted by gardenlet. I.e, HPA _does_ scale down to min replicas but not below min replicas. HPA's max replicas count is 6.

> Note: The `alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled` annotation is alpha and can be removed anytime without further notice. Only use it if you know what you do.

##  Virtual Kubernetes API Server and Gardener API Server

The virtual Kubernetes API server's autoscaling is same as the Shoot Kubernetes API server's with the following differences:
- The initial API server resource requests are `600m` and `512Mi`.
- The min replicas count is 2 for a non-HA virtual cluster and 3 for an HA virtual cluster. The max replicas count is 6.

The Gardener API server's autoscaling is the same as the Shoot Kubernetes API server's with the following differences:
- The initial API server resource requests are `600m` and `512Mi`.
- The min replicas count is 2 for a non-HA virtual cluster and 3 for an HA virtual cluster. The max replicas count is 6.
