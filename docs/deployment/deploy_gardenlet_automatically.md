# Deploy a gardenlet Automatically

The gardenlet can automatically deploy itself into shoot clusters, and register them as seed clusters.
These clusters are called "managed seeds" (aka "shooted seeds").
This procedure is the preferred way to add additional seed clusters, because shoot clusters already come with production-grade qualities that are also demanded for seed clusters.

## Prerequisites

The only prerequisite is to register an initial cluster as a seed cluster that already has a manually deployed gardenlet (for a step-by-step manual installation guide, see [Deploy a Gardenlet Manually](deploy_gardenlet_manually.md)).

> [!TIP]
> The initial seed cluster can be the garden cluster itself, but for better separation of concerns, it is recommended to only register other clusters as seeds.

## Auto-Deployment of Gardenlets into Shoot Clusters

For a better scalability of your Gardener landscape (e.g., when the total number of `Shoot`s grows), you usually need more seed clusters that you can create, as follows:

1. Use the initial seed cluster ("unmanaged seed") to create shoot clusters that you later register as seed clusters.
2. The gardenlet deployed in the initial cluster can deploy itself into the shoot clusters (which eventually makes them getting registered as seeds) if `ManagedSeed` resources are created.

The advantage of this approach is that there's only one initial gardenlet installation required.
Every other managed seed cluster gets an automatically deployed gardenlet.

## Related Links

- [`ManagedSeed`s: Register Shoot as Seed](../operations/managed_seed.md)
