---
title: gardenadm
description: Bootstrapping and management of autonomous shoot clusters.
---

> [!CAUTION]
> This tool is currently under development and considered highly experimental.
> Do not use it in production environments.
> Read more about it in [GEP-28](../proposals/28-autonomous-shoot-clusters.md).

<img src="../../logo/gardenadm-large.png" alt="gardenadm" width="100"/>

## Overview

`gardenadm` is a command line tool for bootstrapping Kubernetes clusters called "Autonomous Shoot Clusters".
In contrast to usual Gardener-managed clusters (called Shoot Clusters), the Kubernetes control plane components run as static pods on a dedicated control plane worker pool in the cluster itself (instead of running them as pods on another Kubernetes cluster (called Seed Cluster)).
Autonomous shoot clusters can be bootstrapped without an existing Gardener installation.
Hence, they can host a Gardener installation itself and/or serve as the initial seed cluster of a Gardener installation.
Furthermore, autonomous shoot clusters can only be created by the `gardenadm` tool and not via an API of an existing Gardener system.

![Architectural overview](../proposals/assets/28-overview.png)

Such autonomous shoot clusters are meant to operate autonomously, but not to exist completely independently of Gardener.
Hence, after their initial creation, they are connected to an existing Gardener system such that the established cluster management functionality via the `Shoot` API can be applied.
I.e., day-2 operations for autonomous shoot clusters are only supported after connecting them to a Gardener system.
This Gardener system could also run in an autonomous shoot cluster itself (in this case, you would first need to deploy it before being able to connect the autonomous shoot cluster to it).

Furthermore, autonomous shoot clusters are not considered a replacement or alternative for regular shoot clusters.
They should be only used for special use-cases or requirements as creating them is more complex and as their costs will most likely be higher (since control plane nodes are typically not fully utilized in such architecture).
In this light, a high cluster creation/deletion churn rate is neither expected nor in scope.

## Getting Started Locally

[This document](../deployment/getting_started_locally_with_gardenadm.md) walks you through deploying Autonomous Shoot Clusters using `gardenadm` on your local machine.
This setup can be used for trying out and developing `gardenadm` locally without additional infrastructure.
The setup is also used for running e2e tests for `gardenadm` in CI.

## Scenarios

We distinguish between two different scenarios for bootstrapping autonomous shoot clusters:

- High Touch, meaning that there is no programmable infrastructure available.
  We consider this the "bare metal" or "edge" use-case, where at first machines must be (often manually) prepared by human operators.
  In this case, network setup (e.g., VPCs, subnets, route tables, etc.) and machine management are out of scope.
- Medium Touch, meaning that there is programmable infrastructure available where we can leverage [provider extensions](../../extensions/README.md#infrastructure-provider) and [`machine-controller-manager`](https://github.com/gardener/machine-controller-manager) in order to manage the network setup and the machines.

The general procedure of bootstrapping an autonomous shoot cluster is similar in both scenarios.
