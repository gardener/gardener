# Deploying Gardenlets

Gardenlets act as decentralized agents to manage the shoot clusters of a seed cluster.

## Procedure

There are two ways of deploying gardenlets:

1. Manually install the [Helm chart](../../charts/gardener/gardenlet).
   After you have deployed the Gardener control plane, you need a dedicated seed cluster (or register the cluster in which the control plane runs).
   This method is typically needed to get a first seed cluster up (so-called "unmanaged seeds").
   It may also be needed if you want to register a cluster as seed that resides behind a firewall.
   For more information, see [Deploy a Gardenlet Manually](deploy_gardenlet_manually.md).
2. Create `ManagedSeed` resources to make existing shoot clusters getting registered as seeds.
   For more information, see [Deploy a gardenlet Automatically](deploy_gardenlet_automatically.md).
