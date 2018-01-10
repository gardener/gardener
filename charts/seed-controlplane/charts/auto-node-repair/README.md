# aws-auto-node-repair

[The auto node repair on AWS](https://github.com/gardener/auto-node-repair) repairs unhealthy nodes by scaling auto scaling groups to create new VMs for each unhealthy node, drain the unhealthy nodes and finally delete them.

Node repairs for multiple nodes occur in parallel for now, unless the autoscaling groups reach its maximum in which case it is bounded by the maximum ASG size.

It is inspired and reuses many components from the cluster-autoscaler

## Introduction

This chart bootstraps an auto-node-repair deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites
  - Kubernetes 1.3+ with Beta APIs enabled
