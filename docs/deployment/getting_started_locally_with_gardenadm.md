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
make gardenadm-up
```

This will first build the needed images, deploy 2 machine pods using the [`gardener-extension-provider-local-node` image](../../pkg/provider-local/node), install the `gardenadm` binary on both of them, and copy the needed manifests to the `/gardenadm/resources` directory.

Afterward, you can use `kubectl exec` to execute `gardenadm` commands on the machines.

Let's start with exec'ing into the `machine-0` pod:

```shell
$ kubectl -n gardenadm-high-touch exec -it machine-0 -- bash
root@machine-0:/# gardenadm -h
gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.
...

root@machine-0:/# cat /gardenadm/resources/manifests.yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
...
```

### Bootstrapping a Single-Node Control Plane

Use `gardenadm init` to bootstrap the first control plane node using the provided manifests:

```shell
root@machine-0:/# gardenadm init -d /gardenadm/resources
...
Your Shoot cluster control-plane has initialized successfully!
...
```

### Connecting to the Autonomous Shoot Cluster

The machine pod's shell environment is configured for easily connecting to the autonomous shoot cluster.
Just execute `kubectl` within a `bash` shell in the machine pod:

```shell
$ kubectl -n gardenadm-high-touch exec -it machine-0 -- bash
root@machine-0:/# kubectl get node
NAME        STATUS   ROLES    AGE     VERSION
machine-0   Ready    <none>   4m11s   v1.32.0
```

You can also copy the kubeconfig to your local machine and use a port-forward to connect to the cluster's API server:

```shell
$ kubectl -n gardenadm-high-touch exec -it machine-0 -- cat /etc/kubernetes/admin.conf | sed 's/api.root.garden.internal.gardenadm.local/localhost:6443/' > /tmp/shoot--garden--root.conf
$ kubectl -n gardenadm-high-touch port-forward pod/machine-0 6443:443

# in a new terminal
$ export KUBECONFIG=/tmp/shoot--garden--root.conf
$ kubectl get no
NAME        STATUS   ROLES    AGE   VERSION
machine-0   Ready    <none>   10m   v1.32.0
```

### Joining a Worker Node

If you would like to join a worker node to the cluster, generate a bootstrap token and the corresponding `gardenadm join` command on `machine-0` (the control plane node).
Then exec into the `machine-1` pod to run the command:

```shell
root@machine-0:/# gardenadm token create --print-join-command
# now copy the output, terminate the exec session and start a new one for machine-1

$ kubectl -n gardenadm-high-touch exec -it machine-1 -- bash
# paste the copied 'gardenadm join' command here and execute it
root@machine-1:/# gardenadm join ...
...
Your node has successfully been instructed to join the cluster as a worker!
...
```

Using the kubeconfig as described in [this section](#connecting-to-the-autonomous-shoot-cluster), you should now be able to see the new node in the cluster:

```shell
$ kubectl get no
NAME        STATUS   ROLES    AGE   VERSION
machine-0   Ready    <none>   10m   v1.32.0
machine-1   Ready    <none>   37s   v1.32.0
```

## Medium-Touch Scenario

Use the following command to prepare the `gardenadm` medium-touch scenario:

```shell
make gardenadm-medium-touch-up
```

This will first build the needed images and then render the needed manifests for `gardenadm bootstrap` to the [`./dev-setup/gardenadm/resources/generated/medium-touch`](../../dev-setup/gardenadm/resources/generated/medium-touch) directory.

Afterward, you can use `go run` to execute `gardenadm` commands on your machine:

```shell
$ export IMAGEVECTOR_OVERWRITE=$PWD/dev-setup/gardenadm/resources/generated/.imagevector-overwrite.yaml
$ go run ./cmd/gardenadm bootstrap -d ./dev-setup/gardenadm/resources/generated/medium-touch
...
Command is work in progress
```

## Connecting the Autonomous Shoot Cluster to Gardener

After you have successfully bootstrapped an autonomous shoot cluster (either via the [high touch](#high-touch-scenario) or the [medium touch](#medium-touch-scenario) scenario), you can connect it to an existing Gardener system.
For this, you need to have a Gardener running locally in your KinD cluster.
In order to deploy it, you can run

```shell
make gardenadm-up SCENARIO=connect
```

This will deploy [`gardener-operator`](../concepts/operator.md) and create a `Garden` resource (which will then be reconciled and results in a full Gardener deployment).
Find all information about it [here](getting_started_locally.md#alternative-way-to-set-up-garden-and-seed-leveraging-gardener-operator).
Note, that in this setup, no `Seed` will be registered in the Gardener - it's just a plain garden cluster without the ability to create regular shoot clusters.

Once above command is finished, you can connect the autonomous shoot cluster to this Gardener instance:

```shell
$ kubectl -n gardenadm-high-touch exec -it machine-0 -- bash
root@machine-0:/# gardenadm connect
2025-07-03T12:21:49.586Z	INFO	Command is work in progress
```

## Running E2E Tests for `gardenadm`

Based on the described setup, you can execute the e2e test suite for `gardenadm`:

```shell
make gardenadm-up SCENARIO=high-touch
make gardenadm-up SCENARIO=medium-touch
make test-e2e-local-gardenadm
```

You can also selectively run the e2e tests for one of the scenarios:

```shell
make gardenadm-up SCENARIO=high-touch
./hack/test-e2e-local.sh gardenadm --label-filter="high-touch" ./test/e2e/gardenadm/...
```

## Tear Down the KinD Cluster

```shell
make kind-down
```
