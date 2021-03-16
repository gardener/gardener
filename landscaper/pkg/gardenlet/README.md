# Gardenlet Landscaper Component

This is a component of the [Gardener Landscaper](https://github.com/gardener/landscaper).
Its purpose is to deploy a Gardenlet to a Kubernetes cluster
and automatically register this cluster as a `Seed` resource in the Gardener installation.

It uses TLS bootstrapping with a bootstrap token to obtain a valid Gardenlet certificate for the Garden cluster.
Essentially, it follows this [documentation](../../../docs/deployment/deploy_gardenlet_manually.md).

## Prerequisites

The gardenlet landscaper deploys a `VerticalPodAutoscaler` resource
**if VPA for the Gardenlet is enabled** (via  field `.deploymentConfiguration.vpa` in the [import configuration](#import-configuration)).
Hence, in this case the Kubernetes cluster to be registered as a Seed must have 
[Vertical Pod Autoscaling](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) enabled 
(requires at least the CRD `VerticalPodAutoscaler` to be registered).

## Run

The Gardenlet landscaper component is supposed to run as part of the [Gardener Landscaper](https://github.com/gardener/landscaper) 
but can also be executed stand-alone.

The Gardenlet landscaper supports the following command line arguments:
- `--integration-test --integration-test[=true|false]` (defaults to `false`).

  In integration tests, we do not assume that the Gardenlet can be rolled out successfully,
  nor that the Seed can be registered.
  This is to provide an easy means of testing the landscaper component without requiring
  a fully functional Gardener control plane.
- `--version version[=true|false|raw]`
  Print version information and quit.
  
Make sure to set the following environment variables:
- `IMPORTS_PATH` contains the path to a file containing [required import configuration](#required-configuration).
- `OPERATION` is set to either `RECONCILE` (deploys the Gardenlet) or `DELETE` to remove the deployed resources.
- `COMPONENT_DESCRIPTOR_PATH` contains the path to a file containing a component descriptor for the Gardenlet. 
   This file contains the OCI image reference to use for the Gardenlet deployment.
   You can find a sample descriptor [here](example/run-locally/gardenlet-landscaper-component-descriptor-list.yaml).
  

### Run locally

1. Run `make dev-setup` to create a default component descriptor and import file for local execution 
in the directory `dev/landscaper`.

2. Adjust the import file `landscaper/gardenlet-landscaper-imports.yaml` to your setup.
Please check what can be configured [below](#import-configuration).

3. Finally, run the below `make` statement to run the Gardenlet landscaper. 
   This already sets the required environment variables and the path to the local imports and component descriptor file.

Run the landscaper gardenlet `RECONCILE` operation:

```
make start-landscaper-gardenlet OPERATION=RECONCILE
```

Run the landscaper gardenlet `DELETE` operation:
```
make start-landscaper-gardenlet OPERATION=DELETE
```

## Import Configuration

### Required configuration

- `seedCluster` contains the kubeconfig for the cluster
   - into which the Gardenlet is deployed by the landscaper
   - that is targeted as the Seed cluster by the Gardenlet via the default in-cluster mounted service account token
   
   Requires `cluster-admin` permissions. 
  
- `gardenCluster` contains the kubeconfig for the Garden cluster. Requires admin permissions to create necessary RBAC roles and secrets.
- `deploymentConfiguration` configures the Kubernetes deployment of the Gardenlet. Specifies parameters, such as the number of replicas, the required resources, etc.
- `componentConfiguration` has to contain a valid [component configuration for the Gardenlet](../../../example/20-componentconfig-gardenlet.yaml).
- `componentConfiguration.seedConfig` must contain the `Seed` resource that will be automatically registered by the Gardenlet. 

### Forbidden configuration 

Because the Gardenlet landscaper only supports TLS bootstrapping with a bootstrap token, setting the field
`.componentConfiguration.gardenClientConnection.kubeconfig` in the import configuration is forbidden.

Deploying the Gardenlet outside the Seed cluster is not supported (e.g., landscaper deploys Gardenlet into
cluster A and the Gardenlet is configured to target cluster B as Seed).
Therefore, setting the field `.componentConfiguration.seedClientConnection.kubeconfig` in the import configuration is forbidden.


### Default values 

The field `.componentConfiguration.gardenClientConnection.kubeconfigSecret` defaults to:

```
kubeconfigSecret:
  name: gardenlet-kubeconfig
  namespace: garden
```

The field `.componentConfiguration.gardenClientConnection.bootstrapKubeconfig` defaults to:

```
bootstrapKubeconfig:
  name: gardenlet-kubeconfig-bootstrap
  namespace: garden
```

All other default values can be found in the [Gardenlet Helm chart](../../../charts/gardener/gardenlet/values.yaml).

### Seed Backup

The setup of etcd backup for the Seed cluster is optional.
You can either
 a) refer to an existing secret in the Garden cluster containing backup credentials using the import configuration field `componentConfiguration.seedConfig.backup`
 b) or use the landscaper component to deploy a secret containing the backup credentials.

For option b), you need to provide the backup provider credentials in
the import configuration field `seedBackupCredentials`.
The Gardenlet landscaper takes care of creating a Kubernetes secret with the name and namespace given in `componentConfiguration.seedConfig.backup.secretRef`
in the Garden cluster containing the credentials given in `seedBackupCredentials`.

### Additional Considerations

If configured in `componentConfiguration.seedConfig.secretRef`, the landscaper will create a secret in 
the Garden cluster containing the kubeconfig of the Seed cluster.
By default, the kubeconfig is taken from `seedCluster.config.kubeconfig`, or if specified, from `.componentConfiguration.seedClientConnection.kubeconfig`.
