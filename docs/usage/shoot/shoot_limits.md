---
title: Shoot Cluster Limits
---

# Shoot Cluster Limits

Gardener operators can configure limits for shoot clusters in the `CloudProfile.spec.limits` section, where they can also be looked up by shoot owners.
The limits are enforced on all shoot clusters using the respective `CloudProfile`.
If a certain limit is not configured, no limit is enforced.

This document explains the limits that can be configured in the `CloudProfile`.

## Maximum Node Count

The `CloudProfile.spec.limits.maxNodesTotal` configures the maximum supported node count of shoot clusters in a Gardener installation.

If this limit is set, Gardener ensures that

- the total minimum node count of all worker pools (i.e., the total initial node count) does not exceed the configured limit
- the maximum node count of an individual worker pool does not exceed the configured limit
- cluster-autoscaler does not provision more nodes than the configured limit (`--max-nodes-total` flag)

The maximum node count of a shoot cluster can be lower than the configured limit, if the cluster's networking configurations don't allow it (see [this doc page](../networking/shoot_networking.md)).

Gardener operators must ensure that no existing shoot cluster exceeds the limit when adding it.
Because Gardener API server itself cannot verify that all shoot clusters would comply with a given limit set in an API request, it does not allow decreasing the limit, which could be disruptive for existing shoots.
Increasing and removing the limits is allowed.

Note that the node count limit during runtime is applied by the cluster-autoscaler only.
E.g., performing a rolling update can cause shoots to exceed `maxNodesTotal` by the total `maxSurge` of all worker pools.
Also, when a shoot owner adds another worker pool to a cluster that has already reached the maximum node count via cluster autoscaling, Gardener would initially deploy the new worker pool with the minimum number of nodes.
This would cause the shoot to temporarily exceed the configured limit until cluster-autoscaler scales the cluster down again.
In other words, `CloudProfile.spec.limits.maxNodesTotal` doesn't enforce a hard limit, but rather ensures that shoot clusters stay within a reasonable size that the Gardener operator can and wants to support.
Shoot owners should keep the limit configured in the `CloudProfile` in mind when configuring the initial node count of new worker pools.
