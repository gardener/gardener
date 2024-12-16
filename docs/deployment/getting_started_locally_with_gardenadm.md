# Deploying Autonomous Shoot Clusters Locally

> [!CAUTION]
> The `gardenadm` tool is currently under development and considered highly experimental.
> Do not use it in production environments.
> Read more about it in [GEP-28](../proposals/28-autonomous-shoot-clusters.md).

This document walks you through deploying Autonomous Shoot Clusters using `gardenadm` on your local machine.
This setup can be used for trying out and developing `gardenadm` locally without additional infrastructure.
The setup is also used for running e2e tests for `gardenadm` in CI ([Prow](https://prow.gardener.cloud)).

If you encounter difficulties, please open an issue so that we can make this process easier.

## Overview

`gardenadm` is a command line tool for bootstrapping Kubernetes clusters called "Autonomous Shoot Clusters". Read the [`gardenadm` documentation](../concepts/gardenadm.md) for more details on its concepts.

In this guide, we will start a [KinD](https://kind.sigs.k8s.io/) cluster which hosts pods serving as machines for the autonomous shoot cluster – just as for shoot clusters of [provider-local](../extensions/provider-local.md).
The setup supports both the high-touch and medium-touch scenario of `gardenadm`.

Based on [Skaffold](https://skaffold.dev/), the container images for all required components will be built and deployed into the cluster.
This also includes the `gardenadm` CLI, which is installed on the machine pods by pulling the container image and extracting the binary.

## Prerequisites

- Make sure that you have followed the [Local Setup guide](../development/local_setup.md) up until the [Get the sources](../development/local_setup.md#get-the-sources) step.
- Make sure your Docker daemon is up-to-date, up and running and has enough resources (at least `8` CPUs and `8Gi` memory; see [here](https://docs.docker.com/desktop/mac/#resources) how to configure the resources for Docker for Mac).
  > Additionally, please configure at least `120Gi` of disk size for the Docker daemon.

> [!TIP]
> You can clean up unused data with `docker system df` and `docker system prune -a`.

## Setting Up the KinD Cluster

```shell
make kind-up
```

Please see [this documentation section](getting_started_locally.md#setting-up-the-kind-cluster-garden-and-seed) for more details.

All following steps assume that you are using the kubeconfig for this KinD cluster:

```shell
export KUBECONFIG=$PWD/example/gardener-local/kind/local/kubeconfig
```

## High-Touch Scenario

Use the following command to prepare the `gardenadm` high-touch scenario:

```shell
make gardenadm-high-touch-up
```

This will first build the needed images, deploy 2 machine pods using the [`gardener-extension-provider-local-node` image](../../pkg/provider-local/node), and install the `gardenadm` binary on both of them.

Afterward, you can use `kubectl exec` to execute `gardenadm` commands on the machines:

```shell
$ kubectl -n gardenadm-high-touch exec -it machine-0 -- bash
root@machine-0:/# gardenadm -h
gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.
...
```

## Medium-Touch Scenario

Use the following command to prepare the `gardenadm` medium-touch scenario:

```shell
make gardenadm-medium-touch-up
```

This will first build the needed images and then render the needed manifests for `gardenadm bootstrap` to the [`./example/gardenadm-local/medium-touch`](../../example/gardenadm-local/medium-touch) directory.

Afterwards, you can use `go run` to execute `gardenadm` commands on your machine:

```shell
$ go run ./cmd/gardenadm -h
gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.
...
```

## Running E2E Tests For `gardenadm`

Based on the described setup, you can execute the e2e test suite for `gardenadm`:

```shell
make gardenadm-high-touch-up gardenadm-medium-touch-up
make test-e2e-local-gardenadm
```

You can also selectively run the e2e tests for one of the scenarios:

```shell
make gardenadm-high-touch-up
./hack/test-e2e-local.sh gardenadm --label-filter="high-touch" ./test/e2e/gardenadm/...
```

## Tear Down the KinD Cluster

```shell
make kind-down
```
