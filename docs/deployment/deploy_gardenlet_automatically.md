# Automatic Deployment of gardenlets

The gardenlet can automatically deploy itself into shoot clusters, and register a cluster as a seed cluster.
These clusters are called "managed seeds" (aka "shooted seeds").
This procedure is the preferred way to add additional seed clusters, because shoot clusters already come with production-grade qualities that are also demanded for seed clusters.

## Prerequisites

The only prerequisite is to register an initial cluster as a seed cluster that already has a gardenlet deployed in one of the following ways:

* The gardenlet was deployed as part of a Gardener installation using a setup tool (for example, `gardener/garden-setup`).
* The gardenlet was deployed manually (for a step-by-step manual installation guide, see [Deploy a Gardenlet Manually](deploy_gardenlet_manually.md)).

> The initial cluster can be the garden cluster itself.

## Self-Deployment of gardenlets in Additional Managed Seed Clusters

For a better scalability, you usually need more seed clusters that you can create, as follows:

1. Use the initial cluster as the seed cluster for other managed seed clusters. It hosts the control planes of the other seed clusters.
1. The gardenlet deployed in the initial cluster deploys itself automatically into the managed seed clusters.

The advantage of this approach is that there’s only one initial gardenlet installation required. Every other managed seed cluster has a gardenlet deployed automatically.

## Related Links

- [Register Shoot as Seed](../operations/managed_seed.md)
- [garden-setup](http://github.com/gardener/garden-setup)