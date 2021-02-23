# Deploying Gardenlets

Gardenlets act as decentral "agents" to manage shoot clusters of a seed cluster.

To support scaleability in an automated way, gardenlets are deployed automatically. However, you can still deploy gardenlets manually to be more flexible, for example, when shoot clusters that need to be managed by Gardener are behind a firewall. The gardenlet only requires network connectivity from the gardenlet to the Garden cluster (not the other way round), so it can be used to register Kubernetes clusters with no public endpoint. 

## Procedure

1. First, an initial gardenlet needs to be deployed:
   
   * Deploy it manually if you have special requirements. More information: [Deploy a Gardenlet Manually](deploy_gardenlet_manually.md)
   * Let the Gardener installer deploy it automatically otherwise. More information: [Automatic Deployment of Gardenlets](deploy_gardenlet_automatically.md)

1. To add additional seed clusters, it is recommended to use regular shoot clusters. You can do this by creating a `ManagedSeed` resource with a `gardenlet` section as described in [Register Shoot as Seed](../usage/managed_seed.md). 



