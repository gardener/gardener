# Gardener Control Plane Landscaper Component

The  Gardener Control Plane Landscaper Component is an executable to set up a production-ready Gardener Control Plane on any Kubernetes cluster.
Under the hood, the `controlplane` helm chart in `charts/gardener/controlplane` is used.

You need to provide a configuration file with few mandatory values.
Please take a look [here](./configuration.md#mandatory-configuration) for more information.

The tool is designed to run with minimal configuration to bootstrap a new or upgrade an existing installation.  
For a new Gardener installation, all certificates are generated.
Used against an existing Gardener installation, missing mandatory import configuration is complemented with configuration detected in-cluster.
This is limited to
- The CA and TLS serving certificates of the Gardener API Server
- The CA and TLS serving certificates of the Gardener Admission Controller
- The TLS serving certificates of the Gardener Controller manager
- The Gardener identity
- The etcd encryption configuration
- The OpenVPN Diffie-Helllmann key
This is to avoid an accidental re-generation of certificates.

**Please note**: The deployment and component configuration of the Gardener Control Plane is not complemented with configuration from an existing Gardener Installation. 
You have to configure the import configuration equivalent to how you previously deployed Gardener.

All used certificates are exported to the path in the environment variable `EXPORTS_PATH`.

# Prerequisites

1) A kubeconfig for a Kubernetes cluster (`runtime` cluster) to run the Gardener control plane pods (Gardener Extension API server, Gardener Controller Manager, Gardener Scheduler, Gardener Admission Controller)

2) A kubeconfig for a Kubernetes cluster with a Kubernetes API server set up for API aggregation.
- The Gardener API server extends this API server and serves the Gardener resource groups.
- For more information how to configure the aggregation layer see the [Kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer).
- This can be the `runtime` cluster.

3) An existing etcd cluster that is accessible from the runtime cluster to be used by the GardenerExtension API server
- if client authentication is enabled on etcd: need to obtain client credentials (x509 certificate and key) trusted by the etcd cluster.

For each Gardener control plane component, vertical pod autoscaling can be enabled via the import configuration in `<gardenerAPIServer/gardenerControllerManager/gardenerAdmissionController/gardenerScheduler>.deploymentConfiguration.vpa`.
If enabled, a `VerticalPodAutoscaler` resource is deployed. Please make sure the runtime cluster has
[Vertical Pod Autoscaling](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) enabled
(requires at least the CRD `VerticalPodAutoscaler` to be registered).

Instead of vertical autoscaling, [HVPA](https://github.com/gardener/hvpa-controller) can be enabled for the Gardener API Server via the import configuration in field
`gardenerAPIServer.deploymentConfiguration.hvpa.enabled`. Please make sure the runtime cluster has
[HVPA](https://github.com/gardener/hvpa-controller) enabled
(requires at least the [CRD `HVPA`](https://github.com/gardener/hvpa-controller/blob/master/config/crd/output/crds.yaml) in resource group `autoscaling.k8s.io/v1alpha1` to be registered).

# Deployment Models

The Gardener control plane can be set up in two different ways.

### 1) Extending the Runtime Cluster

In this setup,
- the `runtime` cluster hosts the Gardener control plane pods
- the API server of the `runtime` cluster is extended by the Gardener Extension API server, thus serves the Gardener resource groups (Shoot, Seed, ...).
- the Gardener control plane pods are configured against the API server of the `runtime` cluster

Consider this option if
- you can directly **configure and upgrade the API server** itself (e.g. to set up API aggregation) - **this is typically not the case on managed Kubernetes offerings**.
- there are no performance concerns considering the `runtime` cluster's API server will serve both the Gardener API plus hosting other workload.

### 2) Virtual Garden

**Prerequisites**
- A Kubernetes API server deployment configured for API aggregation in the `runtime` cluster.
  This component does not deploy the such API server. It must already exist in the runtime cluster
  and setup for [API aggregation](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer).
  This is the case when the [virtual garden component](https://github.com/gardener/virtual-garden) of the Landscaper has already been deployed to the `runtime` cluster.

In this setup,
- the `runtime` cluster hosts the Gardener control plane pods
- a dedicated Kubernetes API server deployed in the `runtime` cluster is extended by the Gardener Extension API server (`virtual-garden` API server).
    - the `virtual-garden` API server serves the Gardener API
- the Gardener control plane pods are configured against the `virtual-garden` API server

Consider this option if
- you want to use a dedicated etcd only for Gardener resource groups
- the etcd of the `runtime` cluster is not under you own control (like on managed Kubernetes offerings)
    - you might want to set up the etcd deployment for scale, deploy an automatic backup solution, etc. (This should be already done when using the [virtual garden component](https://github.com/gardener/virtual-garden))
- scalability of the API server of the runtime cluster is a concern
    - deployed as a dedicated deployment, the virtual Garden API server can scale independently of the `runtime` cluster's API server


Please note that all resources are installed into the `garden` namespace.

# Run

There are two ways to run the Landscaper control plane component.
- execute locally by running a make target,
- deploy with Landscaper.

## Run locally

1. Run `make dev-setup` to create a default component descriptor and import file for local execution
   in the directory `dev/landscaper`.

2. Provide at least the mandatory configuration in the file `dev/landscaper/landscaper-controlpane-imports.yaml` according to the [quick-start configuration](./configuration.md#quick-start-minimal-example-configuration).
   
If you do not have a virtual-garden installation, you can follow [this](https://github.com/gardener/virtual-garden/blob/master/docs/deploy-virtual-garden-with-make-target.md) guide first to set it up on the runtime cluster using the virtual garden Landscaper component.
You can then use the exported `virtual-garden` kubeconfig, etcd URL and certificates to supply the mandatory configuration.
This is how the exports from the virtual-garden Landscaper component map to the import configuration of the Gardener Control Plane Landscaper component:
   
| Virtual Garden Exports | Control Plane Import Configuration  |
|---|---|
| `kubeconfigYaml`  |  `virtualGardenCluster` |
|  `etcdCaPem` |  `etcdCABundle` |
|  `etcdClientTlsPem`  | `etcdClientCert`  |
|  `etcdClientTlsKeyPem` | `etcdClientKey`  |
| virtual-garden-etcd-main-client.garden.svc:2379  |  `etcdUrl` |

4. Finally, run the below `make` statement.
   This already sets the required environment variables and the path to the local imports and component descriptor file.

Run the `RECONCILE` operation:

```
make start-landscaper-control-plane OPERATION=RECONCILE
```

Run the `DELETE` operation:
```
make start-landscaper-control-plane OPERATION=DELETE
```

Alternatively, run using an OCI image. 
For example using the docker CLI from the root directory of the Gardener repository.

```
GARDENER_HOME_DIR=$(pwd)
docker run -v $GARDENER_HOME_DIR/dev/landscaper:/imports  \
-v $GARDENER_HOME_DIR/dev/landscaper/landscaper-controlplane-component-descriptor-list.yaml:/component_descriptor \
-e IMPORTS_PATH=/imports/landscaper-controlplane-imports.yaml \
-e EXPORTS_PATH=/exports.yaml \
-e OPERATION=RECONCILE \
-e COMPONENT_DESCRIPTOR_PATH=/component_descriptor \
eu.gcr.io/gardener-project/gardener/landscaper-control-plane:latest
```

## Deploy using the landscaper

TODO