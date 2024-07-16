---
title: Shoot Hibernation
description: What is hibernation? Manual hibernation/wake up and specifying a hibernation schedule
weight: 2
---

# Shoot Hibernation

Clusters are only needed 24 hours a day if they run productive workload. So whenever you do development in a cluster, or just use it for tests or demo purposes, you can save a lot of money if you scale-down your Kubernetes resources whenever you don't need them. However, scaling them down manually can become time-consuming the more resources you have. 

Gardener offers a clever way to automatically scale-down all resources to zero: cluster hibernation. You can either hibernate a cluster by pushing a button, or by defining a hibernation schedule.

> To save costs, it's recommended to define a hibernation schedule before the creation of a cluster. You can hibernate your cluster or wake up your cluster manually even if there's a schedule for its hibernation.

## What Is Hibernation?

When a cluster is hibernated, Gardener scales down the worker nodes and the cluster's control plane to free resources at the IaaS provider. This affects:

* Your workload, for example, pods, deployments, custom resources.
* The virtual machines running your workload.
* The resources of the control plane of your cluster.

### What Isn’t Affected by the Hibernation?

To scale up everything where it was before hibernation, Gardener doesn’t delete state-related information, that is, information stored in persistent volumes. The cluster state as persistent in `etcd` is also preserved.

## Hibernate Your Cluster Manually

The `.spec.hibernation.enabled` field specifies whether the cluster needs to be hibernated or not. If the field is set to `true`, the cluster's desired state is to be hibernated. If it is set to `false` or not specified at all, the cluster's desired state is to be awakened.

To hibernate your cluster, you can do it from the Gardener dashboard or run the following `kubectl` command:
```
$ kubectl patch shoot -n $NAMESPACE $SHOOT_NAME -p '{"spec":{"hibernation":{"enabled": true}}}'
```

## Wake Up Your Cluster Manually

To wake up your cluster, you can do it from the Gardener dashboard or run the following `kubectl` command:
```
$ kubectl patch shoot -n $NAMESPACE $SHOOT_NAME -p '{"spec":{"hibernation":{"enabled": false}}}'
```

## Create a Schedule to Hibernate Your Cluster

You can specify a hibernation schedule to automatically hibernate/wake up a cluster.

Let's have a look into the following example:

```yaml
  hibernation:
    enabled: false
    schedules:
    - start: "0 20 * * *" # Start hibernation every day at 8PM
      end: "0 6 * * *"    # Stop hibernation every day at 6AM
      location: "America/Los_Angeles" # Specify a location for the cron to run in
```

The above section configures a hibernation schedule that hibernates the cluster every day at 08:00 PM and wakes it up at 06:00 AM. The `start` or `end` fields can be omitted, though at least one of them has to be specified. Hence, it is possible to configure a hibernation schedule that only hibernates or wakes up a cluster. The `location` field is the time location used to evaluate the cron expressions.
