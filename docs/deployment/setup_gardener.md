# Deploying Gardener into a Kubernetes Cluster

Similar to Kubernetes, Gardener consists out of control plane components (Gardener API server, Gardener controller manager, Gardener scheduler), and an agent component (gardenlet).
The control plane is deployed in the so-called garden cluster, while the agent is installed into every seed cluster.
Please note that it is possible to use the garden cluster as seed cluster by simply deploying the gardenlet into it.

We are providing [Helm charts](../../charts/gardener) in order to manage the various resources of the components.
Please always make sure that you use the Helm chart version that matches the Gardener version you want to deploy.

## Deploying the Gardener Control Plane (API Server, Admission Controller, Controller Manager, Scheduler)

In order to deploy the control plane components, please first deploy [`gardener-operator`](../concepts/operator.md#deployment) and create a [`Garden` resource](../concepts/operator.md#garden-resources).

> [!CAUTION]
> Below approach is deprecated and will be removed after v1.135 of Gardener has been released (around beginning of 2026).

The [configuration values](../../charts/gardener/controlplane/values.yaml) depict the various options to configure the different components.
Please consult [Gardener Configuration and Usage](../operations/configuration.md) for component specific configurations and [Authentication of Gardener Control Plane Components Against the Garden Cluster](./authentication_gardener_control_plane.md) for authentication related specifics.

Also, note that all resources and deployments need to be created in the `garden` namespace (not overrideable).
If you enable the Gardener admission controller as part of you setup, please make sure the `garden` namespace is labelled with `app: gardener`.
Otherwise, the backing service account for the admission controller Pod might not be created successfully.
No action is necessary if you deploy the `garden` namespace with the Gardener control plane Helm chart.

After preparing your values in a separate `controlplane-values.yaml` file ([values.yaml](../../charts/gardener/controlplane/values.yaml) can be used as starting point), you can run the following command against your garden cluster:

```bash
helm install charts/gardener/controlplane \
  --namespace garden \
  --name gardener-controlplane \
  -f controlplane-values.yaml \
  --wait
```

## Deploying Gardener Extensions

Gardener is an extensible system that does not contain the logic for provider-specific things like DNS management, cloud infrastructures, network plugins, operating system configs, and many more.

You have to install extension controllers for these parts.
Please consult [the documentation regarding extensions](../extensions/overview.md) to get more information.

## Deploying the Gardener Agent (gardenlet)

Please refer to [Deploying Gardenlets](./deploy_gardenlet.md) on how to deploy a gardenlet.
