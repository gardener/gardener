# Adding Cloud Providers

This document provides an overview of how to integrate a new cloud provider into Gardener. Each component that requires integration has a detailed description of how to integrate it and the steps required.

## Cloud Components

Gardener is composed of 2 or more Kubernetes clusters:

* Shoot: These are the end-user clusters, the regular Kubernetes clusters you have seen. They provide places for your workloads to run.
* Seed: This is the "management" cluster. It manages the control planes of shoots by running them as native Kubernetes workloads.

These two clusters can run in the same cloud provider, but they do not need to. For example, you could run your Seed in AWS, while having one shoot in Azure, two in Google, two in Alicloud, and three in Packet.

The Seed cluster deploys and manages the Shoot clusters. Importantly, for this discussion, the `etcd` data store backing each Shoot runs as workloads inside the Seed. Thus, to use the above example, the clusters in Azure, Google, Alicloud and Packet will have their worker nodes and master nodes running in those clouds, but the `etcd` clusters backing them will run as separate [deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) in the Seed Kubernetes cluster on AWS.

This distinction becomes important when preparing the integration to a new cloud provider.

## Gardener Cloud Integration

Gardener and its related components integrate with cloud providers at the following key lifecycle elements:

* Create/destroy/get/list machines for the Shoot
* Create/destroy/get/list infrastructure components for the Shoot, e.g. VPCs, subnets, routes, etc.
* Backup/restore etcd for the Seed via writing files to and reading them from object storage

Thus, the integrations you need for your cloud provider depend on whether you want to deploy Shoot clusters to the provider, Seed or both.

* Shoot Only: machine lifecycle management, infrastructure.
* Seed: etcd backup/restore

## Gardener API

In addition to the requirements to integrate with the cloud provider, you also need to enable the core Gardener app to receive, validate and process requests to use that cloud provider.

* Expose the cloud provider to the consumers of the Gardener API, so it can be told to use that cloud provider as an option
* Validate that API as requests come in
* Write cloud provider specific implementation (called Botanist)

## Cloud Provider API Requirements

In order for a cloud provider to integrate with Gardener, the provider must have an API to perform machine lifecycle events, specifically:

* Create a machine
* Destroy a machine
* Get information about a machine and its state
* List machines

In addition, if the Seed is to run on the given provider, it also must have an API to save files to block storage and retrieve them, for etcd backup/restore.

The current integration with cloud providers is to add their API calls to Gardener and the Machine Controller Manager. As both Gardener and the Machine Controller Manager are written in [go](https://golang.org), the cloud provider should have a go SDK. However, if it has an API that is wrappable in go, e.g. a REST API, then you can use that to integrate.

The Gardener team is working on bringing cloud provider integrations out-of-tree, making them pluggable, which should simplify the process and make it possible to use other SDKs.

## Summary

To add a new cloud provider, you need some or all of the following. Each repository contains instructions on how to extend it to a new cloud provider.

|Type|Purpose|Location|Documentation|
|---|---|---|---|
|Seed or Shoot|Machine Lifecycle|[machine-controller-manager](https://github.com/gardener/machine-controller-manager)| [MCM new cloud provider](https://github.com/gardener/machine-controller-manager/blob/master/docs/development/new_cp_support.md) |
|Seed only|etcd backup/restore|[etcd-backup-restore](https://github.com/gardener/etcd-backup-restore/)| In process |
|All|Gardener Shoot API extension and validation|[gardener](https://github.com/gardener/gardener)| [Gardener API extension](./new-cloud-provider-api-extension.md) |
|All|Botanist implementation|[gardener](https://github.com/gardener/gardener)| [Botanist cloud provider](./new-cloud-provider-botanist.md) |

