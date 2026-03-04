# Deploying Gardener Remotely

This document will walk you through deploying Gardener on a remote Kubernetes cluster like a Gardener shoot.
It is supposed to run your local Gardener developments on a real infrastructure. For running Gardener locally, please check the [getting started locally](getting_started_locally.md) documentation.
If you encounter difficulties, please open an issue so that we can make this process easier.

## Overview

In this guide, we will use a Kubernetes cluster which is used as garden runtime and soil cluster (please refer to the [architecture overview](../concepts/architecture.md)).
This guide is tested for using Kubernetes clusters provided by Gardener, AWS, Azure, and GCP so far.

Based on [Skaffold](https://skaffold.dev/), the container images for all required components will be built and deployed into the cluster.
This guide leverages `gardener-operator` to manage the Gardener components.

## Prerequisites

- Make sure that you have access to a Kubernetes cluster you can use as garden runtime and soil cluster in this setup.
    - In many cases the resource utilization of the cluster will be best when you use machines with 1:4 CPU to memory ratio. For example, machines with 4 vCPUs and 16 GB of memory could be a good choice for the cluster. 
    - Your cluster must use one single CPU architecture since Skaffold does not build multi-arch images. The CPU architecture of your cluster could still be different from the CPU architecture of your local machine though.
    - You could use any Kubernetes cluster for your garden runtime and soil cluster. However, using a Gardener shoot cluster simplifies some configuration steps.
    - When bootstrapping `gardenlet` to the cluster, your new soil will have the same provider type as the shoot cluster you use - an AWS shoot will become an AWS soil, a GCP shoot will become a GCP soil, etc. (only relevant when using a Gardener shoot as seed).

## Provide Infrastructure Credentials and Configuration

As this setup is running on a real infrastructure, you have to provide credentials for DNS, the infrastructure, and the kubeconfig for the Kubernetes cluster you want to use as seed.

> [!WARNING]
> There are `.gitignore` entries for all files and directories which include credentials. Nevertheless, please double check and make sure that credentials are not committed to the version control system.

### DNS

Gardener control plane requires DNS for the virtual garden, default and internal domains. Thus, you have to configure a valid DNS provider credentials for your setup.

- The DNS credentials for the virtual garden domain should be maintained at [`/dev-setup/garden/overlays/remote/secret-dns.yaml`](/dev-setup/garden/overlays/remote/secret-dns.yaml)
- For the default and internal domains, you have two options:
  - In case DNS credentials based on workload identities are used, `WorkloadIdentity`s should be maintained at [`/dev-setup/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml).
  - If static DNS credentials are used, the `Secret`s for default and internal domains should be maintained at [`/dev-setup/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml).

There are templates with `.tmpl` suffixes for the files in the corresponding folders.

### Projects

There the `garden` and `remote` projects which are predefined and where infrastructure credentials can be created automatically.
Please use the `remote` project for your shoots. Other projects can be created manually.

### Infrastructure

This section explains how to maintain infrastructure credentials for the `remote` and the `garden` project.
If you don't plan to create additional seeds, the `garden` project secrets could remain empty.

In case infrastructure credentials based on workload identities are used, update the `WorkloadIdentity`s in the two files:

- `garden` project: [`/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-workloadidentities.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-workloadidentities.yaml)
- `remote` project: [`/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-workloadidentities.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-workloadidentities.yaml)

If static credentials are used, update the `Secret`s in the following files:

- `garden` project: [`/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-secrets.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-secrets.yaml)
- `remote` project: [`/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-secrets.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-secrets.yaml)

`CredentialsBinding`s for both scenarios should be maintained at:
- `garden` project: [`/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-garden/credentialsbindings.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-garden/credentialsbindings.yaml)
- `remote` project: [`/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-remote/credentialsbindings.yaml`](/dev-setup/gardenconfig/overlays/remote/credentials/credentials-project-remote/credentialsbindings.yaml).

There are templates with `.tmpl` suffixes for the files in the corresponding folders.

### Garden Runtime and Soil Cluster Preparation

The `kubeconfig` of your Kubernetes cluster you would like to use as seed should be placed at [`/dev-setup/remote/kubeconfigs/kubeconfig`](/dev-setup/remote/kubeconfigs/kubeconfig).

> ℹ️ 
> Do **not** use a kubeconfig created with `gardenctl`. It leads to strange errors when skaffold is running kubectl commands.

Additionally, please maintain the configuration of your garden in [`/dev-setup/garden/overlays/remote/garden.yaml`](/dev-setup/garden/overlays/remote/garden.yaml)
and for your gardenlet in [`/dev-setup/gardenlet/overlays/remote/gardenlet.yaml`](/dev-setup/gardenlet/overlays/remote/gardenlet.yaml).
They are automatically copied from their corresponding `*.yaml.tmpl` files in the same directory when you run `make remote up` for the first time. They also include explanations of the properties you should set.

Using a Gardener Shoot cluster as seed simplifies the process, because some configuration options can be taken from `shoot-info` and creating DNS entries and TLS certificates is automated.

### Extensions

You might plan to deploy and register external extensions for networking, operating system, providers, etc. Please put `Extension`s into the [`/dev-setup/extensions/remote`](/dev-setup/extensions/remote) directory. The whole content of this folder will be applied to your garden runtime cluster.
Most likely, you will need at least one networking, one provider and one os extension for your setup.

### `CloudProfile`s

There are no demo `CloudProfiles` yet. Thus, please copy `CloudProfiles` from another landscape to the [`/dev-setup/gardenconfig/overlays/remote/cloudprofile/cloudprofiles.yaml`](/dev-setup/gardenconfig/overlays/remote/cloudprofile/cloudprofiles.yaml)
file or create your own `CloudProfiles` based on the [gardener examples](../../example/30-cloudprofile.yaml). Please check the GitHub repository of your desired provider-extension. Most of them include example `CloudProfile`s.

## Prepare Your Remote Cluster and Configure Your Environment

```bash
make remote-up
```

This command will first prepare the basic configuration of your gardener runtime and soil cluster.

When it starts it will generate config files from corresponding `*.yaml.tmpl` files if they do not exist yet.
Additionally, it checks certain configuration options. If they are not set, it will return an error. Please set those options and run the command again.
When the configuration is correct, the command will proceed with preparing your remote cluster.
It deploys container registry into your remote cluster where all images of the subsequent steps will be pushed to.
Finally, it copies the kubeconfig of your remote cluster to [`/dev-setup/kubeconfigs/runtime/kubeconfig`](/dev-setup/kubeconfigs/runtime/kubeconfig).

The command also sets the `remote` scenario for the other `make` commands. 
Everytime you would like to target your remote cluster again after you ran a different local scenario, please make sure to run `make remote-up` first.

If support for workload identity is required you can invoke the top command with `DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT` variable set to `true`.
External systems can be then configured to trust the workload identity issuer of the remote Garden cluster.

```bash
DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT=true make remote-up
```

Gardener Discovery Server is automatically deployed by Gardener Operator. To setup workload identity with your provider please refer to the provider extension specific docs:

- [provider-aws](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#aws-workload-identity-federation)
- [provider-azure](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#azure-workload-identity-federation)
- [provider-gcp](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#gcp-workload-identity-federation)


## Setting Up Gardener (Garden and Seed on Gardener Cluster)

```bash
make operator-up
```

This command will build the container images for Gardener components, push them to the container registry in your remote
cluster and deploy the `gardener-operator` which will take care of deploying the Gardener components.
Additionally, it will deploy the `Extension`s you defined to the garden runtime cluster.

```bash
make garden-up
```

This command will create the `Garden` resource you prepared before. Finally, it copies the kubeconfig of your virtual garden
to [`/dev-setup/kubeconfigs/virtual-garden/kubeconfig`](/dev-setup/kubeconfigs/virtual-garden/kubeconfig).

```bash
make seed-up
```

This command will deploy the `Gardenlet` resource you prepared before into your virtual garden. This will bootstrap the
soil into your runtime cluster. When this is done, you have a combined garden runtime and soil cluster running in your remote cluster.
You can now start deploying shoots into your virtual garden.

### Rotate credentials of container image registry

There is a container image registry in your garden runtime and soil cluster where Gardener images required for the Garden, Seed and the Shoot nodes are pushed to.
This registry is password protected. The password is generated when remote cluster is prepared via `make remote-up`. Afterward, it is not rotated automatically.
Otherwise, this could break the update of `gardener-node-agent`, because it might not be able to pull its own new image anymore.
This is no general issue of `gardener-node-agent`, but a limitation `remote` setup. Gardener does not support protected container images out of the box. The function was added for this scenario only.

However, if you want to rotate the credentials for any reason, there are two options for it.

- run `make operator-up garden-up seed-up` (to ensure that your images are up-to-date)
- `reconcile` all shoots on the seed where you want to rotate the registry password
- run `kubectl delete secrets -n registry registry-password` on your seed cluster
- run `make remote-up`
- `reconcile` the shoots again

or

- `reconcile` all shoots on the seed where you want to rotate the registry password
- run `kubectl delete secrets -n registry registry-password` on your seed cluster
- run `./dev-setup/remote/registry/deploy-registry.sh <path to runtime kubeconfig> <registry hostname> <path to virtual garden kubeonfig>`
- `reconcile` the shoots again

## Creating a `Shoot` Cluster

You can wait for the `Seed` to be ready by running:

```bash
kubectl wait --for=condition=gardenletready seed remote --timeout=5m
```

`make seed-up` already includes such a check. However, it might be useful when you wake up your remote cluster from hibernation.

Alternatively, you can run `kubectl get seed remote` and wait for the `STATUS` to indicate readiness:

```bash
NAME                  STATUS   PROVIDER   REGION         AGE    VERSION      K8S VERSION
remote   Ready    gcp        europe-west1   111m   v1.137.0-dev   v1.34.7
```

In order to create a first shoot cluster, please create your own `Shoot` definition and apply it to your virtual garden cluster.
`gardener-scheduler` includes `candidateDeterminationStrategy: MinimalDistance` configuration so you are able to run schedule `Shoot`s of different providers on your `Seed`.

You can wait for your `Shoot`s to be ready by running `kubectl -n garden-local get shoots` and wait for the `LAST OPERATION` to reach `100%`. The output depends on your `Shoot` definition. This is an example output:

```bash
NAME        CLOUDPROFILE   PROVIDER   REGION         K8S VERSION   HIBERNATION   LAST OPERATION               STATUS    AGE
aws         aws            aws        eu-west-1      1.34.3        Awake         Create Processing (43%)      healthy   84s
aws-arm64   aws            aws        eu-west-1      1.34.3        Awake         Create Processing (43%)      healthy   65s
azure       az             azure      westeurope     1.34.2        Awake         Create Processing (43%)      healthy   57s
gcp         gcp            gcp        europe-west1   1.34.3        Awake         Create Processing (43%)      healthy   94s
```

### Accessing the `Shoot` Cluster

Your shoot clusters will have a public DNS entries for their API servers, so that they could be reached via the Internet via `kubectl` after you have created their `kubeconfig`.

We encourage you to use the [adminkubeconfig subresource](/docs/proposals/16-adminkubeconfig-subresource.md) for accessing your shoot cluster. You can find an example how to use it in [Accessing Shoot Clusters](/docs/usage/shoot/shoot_access.md#shootsadminkubeconfig-subresource).

## Deleting the `Shoot` Clusters

Before tearing down your environment, you have to delete your shoot clusters. This is highly recommended because otherwise you would leave orphaned items on your infrastructure accounts.

```bash
./hack/usage/delete shoot <your-shoot> garden-local
```

## Tear Down the Gardener Environment

Before you delete your remote cluster, you should shut down your `Shoots`, `Seed` and `Garden` in a clean way to avoid orphaned infrastructure elements in your projects.

Please ensure that your remote cluster is online (not paused or hibernated), all `Shoots` are deleted and run:

```bash
make seed-down garden-down operator-down remote-down
```

This will first uninstall `gardenlet` of your Soil, then destroy your `Garden`, then the `gardener-operator` and
finally the additional components like container registry, etc., are deleted from both your remote cluster.
