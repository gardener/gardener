# Deploying the Gardener into a Kubernetes cluster

Similar to Kubernetes, Gardener consists out of control plane components (Gardener API server, Gardener controller manager, Gardener scheduler), and an agent component (Gardenlet).
The control plane is deployed in the so-called garden cluster while the agent is installed into every seed cluster.
Please note that it is possible to use the garden cluster as seed cluster by simply deploying the Gardenlet into it.

We are providing [Helm charts](../../charts/gardener) in order to manage the various resources of the components.
Please always make sure that you use the Helm chart version that matches the Gardener version you want to deploy.

## Deploying the Gardener control plane (API server, admission controller, controller manager, scheduler)

The [configuration values](../../charts/gardener/controlplane/values.yaml) depict the various options to configure the different components.
Please consult [this document](../usage/configuration.md) to get a detailed explanation of what can be configured for which component.

Also note that all resources and deployments need to be created in the `garden` namespace (not overrideable).
If you enable the Gardener admission controller as part of you setup, please make sure the `garden` namespace is labelled with `app: gardener`.
Otherwise, the backing service account for the admission controller Pod might not be created successfully.
No action is necessary, if you deploy the `garden` namespace with the Gardener control plane Helm chart.

After preparing your values in a separate `controlplane-values.yaml` file ([values.yaml](../../charts/gardener/controlplane/values.yaml) can be used as starting point), you can run the following command against your garden cluster:

```bash
helm install charts/gardener/controlplane \
  --namespace garden \
  --name gardener-controlplane \
  -f controlplane-values.yaml \
  --wait
```

## Deploying Gardener extensions

Gardener is an extensible system that does not contain the logic for provider-specific things like DNS management, cloud infrastructures, network plugins, operating system configs, and many more.

You have to install extension controllers for these parts.
Please consult [the documentation regarding extensions](../extensions/overview.md) to get more information.

## Deploying the Gardener agent (Gardenlet)

Please refer to [this document](./deploy_gardenlet.md) on how to deploy a Gardenlet.