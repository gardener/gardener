---
title: Kubernetes API Server
---

## Overview

The Kubernetes API Server is a component of the Kubernetes control plane that exposes the Kubernetes API. It is the gateway to the Kubernetes cluster - the central touch point that is accessed by all users, automation, and components in the Kubernetes cluster.

## Shoot Kubernetes API Server Autoscaling

There are three supported autoscaling modes for the Shoot Kubernetes API Server.

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

   The `HVPA` mode is the used autoscaling mode when the `HVPA` feature gate is enabled (and the `VPAAndHPAForAPIServer` feature gate is disabled).

- `VPAAndHPA`

   In `VPAAndHPA` mode, the Shoot Kubernetes API server is scaled simultaneously by VPA on CPU and memory utilization and by HPA - on CPU and memory usage. The gardenlet configures VPA and HPA resources in a such a way that the VPA's `maxAllowed` CPU and memory values are a little smaller than the HPA's average usage target. This allows VPA to scale vertically on the Pod's CPU and memory requests. Once all Pods on average exceed the maxAllowed CPU/memory, HPA is scaling horizontally (by adding a new replica).

   The `VPAAndHPA` mode is introduced to address disadvantages with HVPA: additional component; modifies the deployment triggering unnecessary rollouts; vertical scaling only at max replicas; stuck vertical resource requests when scaling in again; etc.

   The initial API server resource requests are `250m` and `500Mi`.

   The `VPAAndHPA` mode is the used autoscaling mode when the `VPAAndHPAForAPIServer` feature gate is enabled (takes precedence over the `HVPA` feature gate).

The API server's replica count in all scaling modes varies between 2 and 3. The min replicas count of 2 is imposed by the [High Availability of Shoot Control Plane Components](../development/high-availability.md#control-plane-components).

The gardenlet sets the initial API server resource requests only when the Deployment is not found. When the Deployment exists, it is not overwriting the kube-apiserver container resources.
