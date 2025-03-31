---
title: Shoot Cluster Purposes
description: Available Shoot cluster purposes and the behavioral differences between them
---

# Shoot Cluster Purpose

The `Shoot` resource contains a `.spec.purpose` field indicating how the shoot is used, whose allowed values are as follows:

* `evaluation` (default): Indicates that the shoot cluster is for evaluation scenarios.
* `development`: Indicates that the shoot cluster is for development scenarios.
* `testing`: Indicates that the shoot cluster is for testing scenarios.
* `production`: Indicates that the shoot cluster is for production scenarios.
* `infrastructure`: Indicates that the shoot cluster is for infrastructure scenarios (only allowed for shoots in the `garden` namespace).

## Behavioral Differences

The following enlists the differences in the way the shoot clusters are set up based on the selected purpose:

* `testing` shoot clusters **do not** get a monitoring or a logging stack as part of their control planes.
* for `production` and `infrastructure` shoot clusters auto-scaling scale down of the main ETCD is disabled.
* shoot addons like `nginxIngress` and `kubernetesDashboard` can only be enabled for `evaluation` shoot clusters.

There are also differences with respect to how `testing` shoots are scheduled after creation, please consult the [Scheduler documentation](../../concepts/scheduler.md).

## Future Steps

We might introduce more behavioral difference depending on the shoot purpose in the future.
As of today, there are no plans yet.
