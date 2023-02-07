# Deploying Gardener Locally

This document will walk you through deploying Gardener on your local machine.
If you encounter difficulties, please open an issue so that we can make this process easier.

## Overview

Gardener runs in any Kubernetes cluster.
In this guide, we will start a [KinD](https://kind.sigs.k8s.io/) cluster which is used as both garden and seed cluster (please refer to the [architecture overview](../concepts/architecture.md)) for simplicity.

Based on [Skaffold](https://skaffold.dev/), the container images for all required components will be built and deployed into the cluster (via their [Helm charts](https://helm.sh/)).

![Architecture Diagram](content/getting_started_locally.png)

## Alternatives

When deploying Gardener on your local machine you might face several limitations:

- Your machine doesn't have enough compute resources (see [prerequisites](#prerequisites)) for hosting a second seed cluster or multiple shoot clusters.
- Testing Gardener's [IPv6 features](../usage/ipv6.md) requires a Linux machine and native IPv6 connectivity to the internet, but you're on macOS or don't have IPv6 connectivity in your office environment or via your home ISP.

In these cases, you might want to check out one of the following options that run the setup described in this guide elsewhere for circumventing these limitations:

- [remote local setup](../development/getting_started_locally.md#remote-local-setup): deploy on a remote pod for more compute resources
- [dev box on Google Cloud](https://github.com/gardener-community/dev-box-gcp): deploy on a Google Cloud machine for more compute resource and/or simple IPv4/IPv6 dual-stack networking

## Prerequisites

- Make sure that you have followed the [Local Setup guide](../development/local_setup.md) up until the [Get the sources](../development/local_setup.md#get-the-sources) step.
- Make sure your Docker daemon is up-to-date, up and running and has enough resources (at least `8` CPUs and `8Gi` memory; see [here](https://docs.docker.com/desktop/mac/#resources) how to configure the resources for Docker for Mac).
  > Please note that 8 CPU / 8Gi memory might not be enough for more than two `Shoot` clusters, i.e., you might need to increase these values if you want to run additional `Shoot`s.
  > If you plan on following the optional steps to [create a second seed cluster](#optional-setting-up-a-second-seed-cluster), the required resources will be more - at least `10` CPUs and `18Gi` memory.
  Additionally, please configure at least `120Gi` of disk size for the Docker daemon.
  > Tip: You can clean up unused data with `docker system df` and `docker system prune -a`.
- Make sure the `kind` docker network is using the CIDR `172.18.0.0/16`.
  - If the network does not exist, it can be created with `docker network create kind --subnet 172.18.0.0/16`
  - If the network already exists, the CIDR can be checked with `docker network inspect kind  | jq '.[].IPAM.Config[].Subnet'`. If it is not `172.18.0.0/16`, delete the network with `docker network rm kind` and create it with the command above.

## Setting Up the KinD Cluster (Garden and Seed)

```bash
make kind-up
```

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./example/gardener-local/kind/local/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config`, since you will need to target this KinD cluster multiple times.
Alternatively, make sure to set your `KUBECONFIG` environment variable to `./example/gardener-local/kind/local/kubeconfig` for all future steps via `export KUBECONFIG=example/gardener-local/kind/local/kubeconfig`.

All of the following steps assume that you are using this kubeconfig.

Additionally, this command also deploys a local container registry to the cluster, as well as a few registry mirrors, that are set up as a pull-through cache for all upstream registries Gardener uses by default.
This is done to speed up image pulls across local clusters.
The local registry can be accessed as `localhost:5001` for pushing and pulling.
The storage directories of the registries are mounted to the host machine under `dev/local-registry`.
With this, mirrored images don't have to be pulled again after recreating the cluster.

The command also deploys a default [calico](https://github.com/projectcalico/calico) installation as the cluster's CNI implementation with `NetworkPolicy` support (the default `kindnet` CNI doesn't provide `NetworkPolicy` support).
Furthermore, it deploys the [metrics-server](https://github.com/kubernetes-sigs/metrics-server) in order to support HPA and VPA on the seed cluster.

## Setting Up Gardener

```bash
make gardener-up
```

This will first build the base image (which might take a bit if you do it for the first time).
Afterwards, the Gardener resources will be deployed into the cluster.

## Creating a `Shoot` Cluster

Alternatively, you can run `kubectl get seed local` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local   Ready    local      local    4m42s   vX.Y.Z-dev    v1.21.1
```

In order to create a first shoot cluster, just run:

```bash
kubectl apply -f example/provider-local/shoot.yaml
```

You can wait for the `Shoot` to be ready by running:

```bash
NAMESPACE=garden-local ./hack/usage/wait.sh shoot APIServerAvailable ControlPlaneHealthy ObservabilityComponentsHealthy EveryNodeReady SystemComponentsHealthy
```

Alternatively, you can run `kubectl -n garden-local get shoot local` and wait for the `LAST OPERATION` to reach `100%`:

```bash
NAME    CLOUDPROFILE   PROVIDER   REGION   K8S VERSION   HIBERNATION   LAST OPERATION            STATUS    AGE
local   local          local      local    1.21.0        Awake         Create Processing (43%)   healthy   94s
```

(Optional): You could also execute a simple e2e test (creating and deleting a shoot) by running:

```shell
make test-e2e-local-simple KUBECONFIG="$PWD/example/gardener-local/kind/local/kubeconfig"
```

### Accessing the `Shoot` Cluster

⚠️ Please note that in this setup, shoot clusters are not accessible by default when you download the kubeconfig and try to communicate with them.
The reason is that your host most probably cannot resolve the DNS names of the clusters since `provider-local` extension runs inside the KinD cluster (for more details, see [DNSRecord](../extensions/provider-local.md#dnsrecord)).
Hence, if you want to access the shoot cluster, you have to run the following command which will extend your `/etc/hosts` file with the required information to make the DNS names resolvable:

```bash
cat <<EOF | sudo tee -a /etc/hosts

# Manually created to access local Gardener shoot clusters.
# TODO: Remove this again when the shoot cluster access is no longer required.
127.0.0.1 api.local.local.external.local.gardener.cloud
127.0.0.1 api.local.local.internal.local.gardener.cloud

127.0.0.1 api.e2e-managedseed.garden.external.local.gardener.cloud
127.0.0.1 api.e2e-managedseed.garden.internal.local.gardener.cloud
127.0.0.1 api.e2e-hibernated.local.external.local.gardener.cloud
127.0.0.1 api.e2e-hibernated.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-unpriv.local.external.local.gardener.cloud
127.0.0.1 api.e2e-unpriv.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-wake-up.local.external.local.gardener.cloud
127.0.0.1 api.e2e-wake-up.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-migrate.local.external.local.gardener.cloud
127.0.0.1 api.e2e-migrate.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-rotate.local.external.local.gardener.cloud
127.0.0.1 api.e2e-rotate.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-default.local.external.local.gardener.cloud
127.0.0.1 api.e2e-default.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-update-node.local.external.local.gardener.cloud
127.0.0.1 api.e2e-update-node.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-update-zone.local.external.local.gardener.cloud
127.0.0.1 api.e2e-update-zone.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upgrade.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upgrade.local.internal.local.gardener.cloud
EOF
```

Now you can access it by running:

```bash
kubectl -n garden-local get secret local.kubeconfig -o jsonpath={.data.kubeconfig} | base64 -d > /tmp/kubeconfig-shoot-local.yaml
kubectl --kubeconfig=/tmp/kubeconfig-shoot-local.yaml get nodes
```

## (Optional): Setting Up a Second Seed Cluster

There are cases where you would want to create a second cluster seed in your local setup. For example, if you want to test the [control plane migration](../usage/control_plane_migration.md) feature. The following steps describe how to do that.


```bash
make kind2-up
```

This command sets up a new KinD cluster named `gardener-local2` and stores its kubeconfig in the `./example/gardener-local/kind/local2/kubeconfig` file.

In order to deploy required resources in the KinD cluster that you just created, run:

```bash
make gardenlet-kind2-up
```

The following steps assume that you are using the kubeconfig that points to the `gardener-local` cluster (first KinD cluster): `export KUBECONFIG=example/gardener-local/kind/local/kubeconfig`.

Alternatively, you can run `kubectl get seed local2` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local2  Ready    local      local    4m42s   vX.Y.Z-dev    v1.21.1
```

If you want to perform control plane migration, you can follow the steps outlined in [Control Plane Migration](../usage/control_plane_migration.md) to migrate the shoot cluster to the second seed you just created.

## Deleting the `Shoot` Cluster

```shell
./hack/usage/delete shoot local garden-local
```

## (Optional): Tear Down the Second Seed Cluster

``` shell
make kind2-down
```

## Tear Down the Gardener Environment

```shell
make kind-down
```

## Remote Local Setup

Just like Prow is executing the KinD based integration tests in a K8s pod, it is
possible to interactively run this KinD based Gardener development environment,
aka "local setup", in a "remote" K8s pod.

```shell
k apply -f docs/deployment/content/remote-local-setup.yaml
k exec -it deployment/remote-local-setup -- sh

tmux -u a
```

### Caveats

Please refer to the [TMUX documentation](https://github.com/tmux/tmux/wiki) for
working effectively inside the remote-local-setup pod.

To access Grafana, Prometheus or other components in a browser, two port forwards are needed:

The port forward from the laptop to the pod:

```shell
k port-forward deployment/remote-local-setup 3000
```

The port forward in the remote-local-setup pod to the respective component:

```shell
k port-forward -n shoot--local--local deployment/grafana-operators 3000
```

## Related Links

- [Local Provider Extension](../extensions/provider-local.md)
