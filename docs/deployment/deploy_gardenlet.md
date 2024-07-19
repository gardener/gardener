# Deploying Gardenlets

Gardenlets act as decentralized agents to manage the shoot clusters of a seed cluster.

## Procedure

After you have deployed the Gardener control plane, you need one or more seed clusters in order to be able to create shoot clusters.

You can either register an existing cluster as "seed" (this could also be the cluster in which the control plane runs), or you can create new clusters (typically shoots, i.e., this approach registers at least one first initial seed) and then register them as "seeds".

The following documents describe the scenarios:

1. If you have not registered a seed cluster yet (thus, you need to deploy a first, so-called "unmanaged seed"), your approach depends on how you deployed the Gardener control plane:
   1. **Via [`gardener-operator`](../concepts/operator.md)**:
      1. If you want to register the same cluster in which `gardener-operator` runs, or if you want to register another cluster that is reachable (network-wise) for `gardener-operator`, you can follow [Deploy gardenlet via `gardener-operator`](deploy_gardenlet_via_operator.md).
      2. If you want to register a cluster that is not reachable (network-wise) (e.g., because it runs behind a firewall), you can follow [Deploy a gardenlet Manually](deploy_gardenlet_manually.md). 
   2. **Via [`gardener/controlplane` Helm chart](../../charts/gardener/controlplane)**: You can follow [Deploy a gardenlet Manually](deploy_gardenlet_manually.md).
2. If you already have a seed cluster, and you want to deploy further seed clusters (so-called "managed seeds"), you can follow [Deploy a gardenlet Automatically](deploy_gardenlet_automatically.md).
