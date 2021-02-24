# Gardenlet Landscaper Component

This is a component of the [Gardener Landscaper](https://github.com/gardener/landscaper).
Its purpose is to deploy a Gardenlet to a Kubernetes cluster
and automatically register this cluster as a `Seed` resource in the Gardener installation.

Uses TLS bootstrapping with a bootstrap token to obtain a valid Gardenlet certificate for the Garden cluster.
Essentially follows this [documentation](../../docs/deployment/deploy_gardenlet_manually.md).

## Prerequisites

The Kubernetes cluster to be registered as a Seed must have [Vertical Pod Autoscaling](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) enabled
**if VPA for the Gardenlet is enabled** (via  field `.deploymentConfiguration.vpa` in the [import configuration](#import-configuration). 
The landscaper only deployes the `VerticalPodAutoscaler` resource.

## Run

The Gardenlet landscaper component is supposed to run as part of the [Gardener Landscaper](https://github.com/gardener/landscaper) but can also be executed stand-alone.

The Gardenlet landscaper does not support command line arguments.
However, make sure to set the following environment variables:
- `IMPORTS_PATH` contains the path to a file containing [required import configuration](#required-configuration).
- `OPERATION` is set to either `RECONCILE` (deploys the Gardenlet) or `DELETE` to remove the deployed resources.
- `COMPONENT_DESCRIPTOR_PATH` contains the path to a file containing a component descriptor for the Gardenlet. 
   This file contains the OCI image reference to use for the Gardenlet deployment.
   You can find a sample descriptor [here](example/local/component_descriptor_list.yaml).
   
The Gardenlet landscaper can be run locally by executing the below `make` statement.
The file path to a valid landscaper import configuration file has to be provided as the first argument.
The [example file](examplemports.yaml) can be used as a template.

Run the landscaper gardenlet `RECONCILE` operation:
```
make start-landscaper-gardenlet-reconcile
```

Run the landscaper gardenlet `DELETE` operation:
```
make start-landscaper-gardenlet-delete
```

## Import Configuration

### Required configuration

- `seedCluster`contains the kubeconfig for the cluster
   - into which the Gardenlet is deployed by the landscaper
   - that is targeted as the Seed cluster by the Gardenlet via the default in-cluster mounted service account token
   
   Hence, the Gardenlet is always deployed into the Seed cluster itself.
   Requires `cluster-admin` permissions. 
  
- `gardenCluster` contains the kubeconfig for the Garden cluster. Requires admin permissions to create necessary RBAC roles and secrets.
- `deploymentConfiguration` configures the Kubernetes deployment of the Gardenlet. Specifies parameters, such as the number of replicas, the required resources, etc.
- `componentConfiguration` has to contain a valid [component configuration for the Gardenlet](../../example/20-componentconfig-gardenlet.yaml).
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

All other default values can be found in the [Gardenlet Helm chart](../../charts/gardener/gardenlet/values.yaml).

### Seed Backup

The setup of etcd backup for the Seed cluster is optional.
You can either
 a) refer to an existing secret in the Garden cluster containing backup credentials using the import configuration field `componentConfiguration.seedConfig.backup`
 b) or use the landscaper component to deploy a secret containing the backup credentials.

For option b), you need to provide the backup provider and credentials in
the import configuration field `seedBackup`.
Additionally, the field `componentConfiguration.seedConfig.spec.backup` needs to be provided
specifying the matching backup provider and secret reference.
The Gardenlet landscaper takes care of creating a Kubernetes secret with the name and namespace given in `componentConfiguration.seedConfig.backup.secretRef`
in the Garden cluster containing the credentials given in `seedBackup`.

### Additional Considerations

If configured in `componentConfiguration.seedConfig.secretRef`, the landscaper will create a secret in 
the Garden cluster containing the kubeconfig of the Seed cluster.
By default, the kubeconfig is taken from `seedCluster.config.kubeconfig`, or if specified, from `.componentConfiguration.seedClientConnection.kubeconfig`.

