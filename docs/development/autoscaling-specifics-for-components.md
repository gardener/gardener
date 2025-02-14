---
title: Autoscaling Specifics for Components
---

## Overview

This document describes the used autoscaling mechanism for several components.

## Garden or Shoot Cluster etcd

The `etcd` is scaled by a native `VPA` resource.

Downscaling is handled more pessimistically to prevent many subsequent etcd restarts. Thus, for `production` and `infrastructure` Shoot clusters (or all Garden clusters), downscaling is deactivated for the main etcd. For all other Shoot clusters, lower advertised requests/limits are only applied during the Shoot's maintenance time window.

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

> [!NOTE]
> The `alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled` annotation is alpha and can be removed anytime without further notice. Only use it if you know what you do.

##  Virtual Kubernetes API Server and Gardener API Server

The virtual Kubernetes API server's autoscaling is same as the Shoot Kubernetes API server's with the following differences:
- The initial API server resource requests are `600m` and `512Mi`.
- The min replicas count is 2 for a non-HA virtual cluster and 3 for an HA virtual cluster. The max replicas count is 6.

The Gardener API server's autoscaling is the same as the Shoot Kubernetes API server's with the following differences:
- The initial API server resource requests are `600m` and `512Mi`.
- The min replicas count is 2 for a non-HA virtual cluster and 3 for an HA virtual cluster. The max replicas count is 6.

## Configure `minAllowed` Resources for Control Plane Components

It is possible to configure minimum allowed resources (`minAllowed`) for CPU and memory for etcd instances and the Kubernetes API server.
This configuration is available for both Shoot clusters and the Garden cluster.

A primary use-case for configuring `minAllowed` resources arises from the need to alleviate delays during consecutive scale-up activities.
Typically, in longer-running clusters, resource usage patterns evolve gradually, and the control plane can scale vertically in an adequate manner.
However, in the case of newly spun-up clusters requiring immediate heavy usage, setting a `minAllowed` threshold for CPU and memory ensures that the control plane components are provisioned with sufficient resources to handle abrupt load increases without substantial delay.

> [!NOTE]
> To use this feature effectively, users should thoroughly analyze their cluster usage patterns in advance to identify appropriate resource values.
