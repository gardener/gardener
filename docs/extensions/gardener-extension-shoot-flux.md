---
title: Universal Shoot Configuration
---

<!-- markdown-toc start - Don't edit this section. Run M-x markdown-toc-refresh-toc -->
**Table of Contents**

- [Introduction](#introduction)
- [Example use cases](#example-use-cases)
  - [Development](#development)
  - [CI/CD](#cicd)
- [General concept](#general-concept)
- [Example Usage](#example-usage)

<!-- markdown-toc end -->

## Introduction
[Flux](https://fluxcd.io/) offers a set of controllers allowing for reconciling a Kubernetes cluster with a declarative state defined in e.g. a Git repository.
Thus it enables GitOps workflows for Kubernetes clusters.
Moreover, it provides a general approach of deploying software components into Kubernetes clusters.
[Gardener](https://gardener.cloud/) is a multi cloud managed Kubernetes service allowing end users to create clusters with a few clicks in its dashboard.
However, the user will obtain a vanilla Kubernetes cluster and has to take care for all the components to be deployed into it.
Of course, the deployment can be performed manually by applying Kubernetes manifests to the cluster.
On the other hand, tools like Flux can help to keep track of the deployments and automate the overall process.
Thus, the combination of Gardener and Flux features the potential of creating new Kubernetes clusters in a pre-defined state.
For the end users, this results in the seamless creation of clusters with all components on their wish list installed.
The [gardener-extension-shoot-flux](https://github.com/23technologies/gardener-extension-shoot-flux) bridges the gap between Gardener and Flux and allows for reconciliation of `Shoot` clusters to resources defined in a Git repository.
By concept, the extension operates on a per-project basis so that clusters in different projects can be reconciled to different repositories.

## Example use cases

### Development
Imagine you are developing software which will eventually run on a Kubernetes cluster in the public cloud.
Moreover, you and your colleagues want to be able to perform some end-to-end tests besides running your local test suite.
For these end-to-end test, an environment mimicking the final production environment is required.
Therefore, you might need tools like [cert-manager](https://cert-manager.io/) or [MinIO](https://min.io/).
However, you do not want keep several testing clusters in the public cloud available for economic reasons and, in consequence, you need to create new clusters on demand.
In this case, the [gardener-extension-shoot-flux](https://github.com/23technologies/gardener-extension-shoot-flux) comes handy, since it allows to configure the cluster asynchronously.
Put simply, you can define the desired state of your cluster in a Git repository, and the new clusters will be reconciled to this state automatically.
Eventually, this will save the effort to configure the clusters each and every time manually.
Of course, you could achieve something similar by hibernation of the development clusters.
However, in that case you are less flexible, since throwing away the cluster in case you lost track of your clusters state comes at the price of reconfiguring the entire cluster.

### CI/CD
Similar to the development use case above, you might want to run your CI/CD pipeline in Kubernetes clusters coming with a few components already installed.
As your pipeline runs frequently, you want to create clusters on the fly or maybe pre-spawn just a few of them.
In order to keep your pipeline simple, you can use the [gardener-extension-shoot-flux](https://github.com/23technologies/gardener-extension-shoot-flux) for the configuration of your CI/CD clusters.
This way your pipeline can focus on the actual action and does not have to perform the cluster configuration beforehand.
This most probably results in cleaner and more stable CI/CD pipelines.

## General concept
The general concept of this extension is visualized in the block diagram below.
```
                 ┌─────────────────────────────────────────────────────────┐
                 │ Gardener operator                                       │
                 ├─────────────────────────────────────────────────────────┤
                 │ - A human being                                         │
                 │                                                         ├────────────┐
                 │                                                         │            │
                 │                                                         │            │
                 └────────┬────────────────────────────────────────────────┘            │
                          │                           ▲                                 │configures
                          │deploys                    │                                 │SSH-key
                          │Configmap                  │read SSH-key                     │
                          │                           │                                 │
                          ▼                           │                                 │
                 ┌────────────────────────────────────┴───────────────────┐             │
                 │ Garden cluster                                         │             │
                 ├────────────────────────┬─────────────────────────┬─────┤             │
                 │ Projetct 1             │ Project 2               │ ... │             ▼
                 ├────────────────────────┼─────────────────────────┼─────┤  ┌─────────────────────┐
                 │- Configmap containing  │- Configmap containing   │     │  │ Git repository      │
                 │  flux configuration    │  flux configuration     │     │  ├─────────────────────┤
                 │                        │                         │     │  │ - Configuration for │
            ┌───►│- ControllerRegistration│- ControllerRegistration │ ... │  │   shoot clusters    │
            │    │                        │                         │     │  └─────────────────────┘
            │    │- Shoot with extension  │- Shoot with extension   │     │             ▲
            │    │  enabled               │  enabled                │     │             │
            │    │                        │                         │     │             │
read config │    │                        │                         │     │             │
and generate│    └────────────────────────┴─────────────────────────┴─────┘             │reconcile
SSH-keys    │                                                                           │
            │    ┌────────────────────────┐     ┌────────────────────────┐              │
            │    │ Seed cluster           │     │ Shoot cluster          │              │
            │    ├────────────────────────┤     ├────────────────────────┤              │
            │    │- Controller watching   │     │                        │              │
            └────┼─ extension resource    │     │- Flux controllers  ────┼──────────────┘
                 │     │                  │     │                        │
                 │     │deploys           │     │- GitRepository resource│
                 │     │                  │     │                        │
                 │     ▼                  │     │- A main kustomization  │
                 │- Managed resources     │     │                        │
                 │  for flux controllers  │     │                        │
                 │  and flux config       │     │                        │
                 │                        │     │                        │
                 └────────────────────────┘     └────────────────────────┘
```
As depicted, the Gardener operator needs to deploy a `ConfigMap` into the Garden cluster.
This `ConfigMap` holds some configuration parameters for the extension controller.
Moreover, the Gardener operator needs to configure an SSH-key for the Git repository in case of a private repository.
This key can be read from the `Secret` called `flux-source` in the Garden cluster which is created by the extension controller.
Of course, the process of adding the SSH-key to the repository depends on the repository host.
E.g. for repositories hosted on Github, the key can simply be added as "Deploy key" in the web-interface.

The extension controller is running in `Seed` clusters.
Besides generating `Secret`s containing SSH-keys, it reads the configuration from the Garden cluster and creates `Managedresources` to be processed by the [Gardener Resource Manager](https://gardener.cloud/docs/gardener/concepts/resource-manager/#managedresource-controller).
These `Managedresources` entail the resources for the Flux controllers, a [GitRepository](https://fluxcd.io/docs/components/source/gitrepositories/) resource matching the configuration, and a main [Kustomization](https://fluxcd.io/docs/components/kustomize/kustomization/) resource.
Once the Gardener Resource Manager has deployed these resources to the `Shoot` cluster, the Flux controllers will reconcile the cluster to the state defined in the Git repository.

You might wonder how the communication between `Seed` clusters and Garden cluster is established.
This is achieved by making use of the `Secret` containing the `gardenlet-kubeconfig` which should be available, when the gardenlet is run inside the `Seed` cluster.
Most probably, this is not the most elegant solution, but it resulted in a quick first working solution.

## Example Usage
Of course, you need to install the extension before you can use it.
You can find `ControllerRegistration`s on the extension's [Github release page](https://github.com/23technologies/gardener-extension-shoot-flux/releases).
So, you can simply go for
``` shell
export KUBECONFIG=KUBECONFIG-FOR-GARDEN-CLUSTER
kubectl -f https://github.com/23technologies/gardener-extension-shoot-flux/releases/download/v0.1.2/controller-registration.yaml
```
in order to install the extension.

For an exemplary use of the extension, 23Technologies have prepared a public repository containing manifest for the installation of [Podinfo](https://github.com/stefanprodan/podinfo).
As a Gardener operator you can apply the following `ConfigMap` to your Garden cluster
``` yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: flux-config
  namespace: YOUR-PROJECT-NAMESPACE
data:
  fluxVersion: v0.29.5 # optional, if not defined the latest release will be used
  repositoryUrl: https://github.com/23technologies/shootflux.git
  repositoryBranch: main
  repositoryType: public
```
As the repository is public you can create a new `Shoot` now and enable the extension for this `Shoot`.
Take the snipped below as an example.
``` yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: bar
  namespace: garden-foo
spec:
  extensions:
  - type: shoot-flux
...
```
Gardener will take care for the `Shoot` creation process.
As soon as you can, you can fetch the `kubeconfig.yaml` for your new `Shoot` from e.g. the Gardener dashboard.
Now, you can watch this cluster by
``` shell
export KUBECONFIG=KUBECONFIG-FOR-SHOOT
k9s
```
and you should see that a `podinfo` deployment should come up.
Great! You successfully created a `Shoot` with the [gardener-extension-shoot-flux](https://github.com/23technologies/gardener-extension-shoot-flux).
