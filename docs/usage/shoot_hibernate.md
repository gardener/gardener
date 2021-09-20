---
title: Hibernate a Cluster
level: beginner
category: Operation
scope: operator
tags: ["task"]
publishdate: 2020-11-19
---
# Hibernate a Cluster

Clusters are only needed 24 hours a day if they run productive workload. So whenever you do development in a cluster, or just use it for tests or demo purposes, you can save much money if you scale-down your Kubernetes resources whenever you don't need them. However, scaling them down manually can become time-consuming the more resources you have. 

Gardener offers a clever way to automatically scale-down all resources to zero: cluster hibernation. You can either hibernate a cluster by pushing a button or by defining a hibernation schedule.

> To save costs, it's recommended to define a hibernation schedule before the creation of a cluster. You can hibernate your cluster or wake up your cluster manually even if there's a schedule for its hibernation.

- [What is hibernated?](#what-is-hibernated)
- [What isn’t affected by the hibernation?](#what-isnt-affected-by-the-hibernation)
- [Hibernate your cluster manually](#hibernate-your-cluster-manually)
- [Wake up your cluster manually](#wake-up-your-cluster-manually)
- [Create a schedule to hibernate your cluster](#create-a-schedule-to-hibernate-your-cluster)


## What is hibernated?

When a cluster is hibernated, Gardener scales down worker nodes and the cluster's control plane to free resources at the IaaS provider. This affects:

* Your workload, for example, pods, deployments, custom resources.
* The virtual machines running your workload.
* The resources of the control plane of your cluster.

## What isn’t affected by the hibernation?

To scale up everything where it was before hibernation, Gardener doesn’t delete state-related information, that is, information stored in persistent volumes. The cluster state as persistent in `etcd` is also preserved.

## Hibernate your cluster manually

To hibernate your cluster you can run the following `kubectl` command:
```
$ kubectl patch shoot -n $NAMESPACE $SHOOT_NAME -p '{"spec":{"hibernation":{"enabled": true}}}'
```

## Wake up your cluster manually

To wake up your cluster you can run the following `kubectl` command:
```
$ kubectl patch shoot -n $NAMESPACE $SHOOT_NAME -p '{"spec":{"hibernation":{"enabled": false}}}'
```

**Hibernation schedule is also supported. More details can be found [here](../../pkg/apis/core/v1beta1/types_shoot.go#L335-L348)**
