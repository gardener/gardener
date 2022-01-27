# Running Gardener locally

This document will walk you through running Gardener on your local machine for development purposes.
If you encounter difficulties, please open an issue so that we can make this process easier.

Gardener runs in any Kubernetes cluster.
In this guide, we will start a [KinD](https://kind.sigs.k8s.io/) cluster which is used as both garden and seed cluster (please refer to the [architecture overview](../concepts/architecture.md)) for simplicity.

The Gardener components, however, will be run as regular processes on your machine (hence, no container images are being built).

![Architecture Diagram](content/getting_started_locally.png)

## Prerequisites

- Make sure your Docker daemon is up-to-date, up and running and has enough resources (at least `4` CPUs and `4Gi` memory; see [here](https://docs.docker.com/desktop/mac/#resources) how to configure the resources for Docker for Mac).
  > Please note that 4 CPU / 4Gi memory might not be enough for more than one `Shoot` cluster, i.e., you might need to increase these values if you want to run additional `Shoot`s.

  Additionally, please configure at least `120Gi` of disk size for the Docker daemon.
  > Tip: With `docker system df` and `docker system prune -a` you can cleanup unused data.
- Make sure that you increase the maximum number of open files on your host:
  - On Mac, run `sudo launchctl limit maxfiles 65536 200000`
  - On Linux, extend the `/etc/security/limits.conf` file with

    ```text
    * hard nofile 97816
    * soft nofile 97816
    ```

    and reload the terminal.

## Setting up the KinD cluster (garden and seed)

```bash
make kind-up KIND_ENV=local
```

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./example/gardener-local/kind/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config` since you will need to target this KinD cluster multiple times.
Alternatively, make sure to set your `KUBECONFIG` environment variable to `./example/gardener-local/kind/kubeconfig` for all future steps via `export KUBECONFIG=example/gardener-local/kind/kubeconfig`.

All following steps assume that your are using this kubeconfig.

## Setting up Gardener

```bash
make dev-setup                                                                # preparing the environment (without webhooks for now)
kubectl wait --for=condition=ready pod -l run=etcd -n garden --timeout 2m     # wait for etcd to be ready
make start-apiserver                                                          # starting gardener-apiserver
```

In a new terminal pane, run

```bash
kubectl wait --for=condition=available apiservice v1beta1.core.gardener.cloud # wait for gardener-apiserver to be ready
make start-admission-controller                                               # starting gardener-admission-controller
```

In a new terminal pane, run

```bash
make dev-setup DEV_SETUP_WITH_WEBHOOKS=true                                   # preparing the environment with webhooks
make start-controller-manager                                                 # starting gardener-controller-manager
```

(Optional): In a new terminal pane, run

```bash
make start-scheduler                                                          # starting gardener-scheduler
```

In a new terminal pane, run

```bash
make register-local-env                                                       # registering the local environment (CloudProfile, Seed, etc.)
make start-gardenlet SEED_NAME=local                                          # starting gardenlet
```

In a new terminal pane, run

```bash
make start-extension-provider-local                                           # starting gardener-extension-provider-local
```

ℹ️ The [`provider-local`](../extensions/provider-local.md) is started with elevated privileges since it needs to manipulate your `/etc/hosts` file to enable you accessing the created shoot clusters from your local machine, see [this](../extensions/provider-local.md#dnsrecord) for more details.

## Creating a `Shoot` cluster

You can wait for the `Seed` to be ready by running

```bash
kubectl wait --for=condition=gardenletready --for=condition=extensionsready --for=condition=bootstrapped seed local --timeout=5m
```

Alternatively, you can run `kubectl get seed local` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local   Ready    local      local    4m42s   vX.Y.Z-dev    v1.21.1
```

In order to create a first shoot cluster, just run

```bash
kubectl apply -f example/provider-local/shoot.yaml
```

You can wait for the `Shoot` to be ready by running

```bash
kubectl wait --for=condition=apiserveravailable --for=condition=controlplanehealthy --for=condition=everynodeready --for=condition=systemcomponentshealthy shoot local -n garden-local --timeout=10m
```

Alternatively, you can run `kubectl -n garden-local get shoot local` and wait for the `LAST OPERATION` to reach `100%`:

```bash
NAME    CLOUDPROFILE   PROVIDER   REGION   K8S VERSION   HIBERNATION   LAST OPERATION            STATUS    AGE
local   local          local      local    1.21.0        Awake         Create Processing (43%)   healthy   94s
```

(Optional): You could also execute a simple e2e test (creating and deleting a shoot) by running

```shell
make test-e2e-local-fast KUBECONFIG="$PWD/example/gardener-local/kind/kubeconfig"
```

When the shoot got successfully created you can access it as follows:

```bash
kubectl -n garden-local get secret local.kubeconfig -o jsonpath={.data.kubeconfig} | base64 -d > /tmp/kubeconfig-shoot-local.yaml
kubectl --kubeconfig=/tmp/kubeconfig-shoot-local.yaml get nodes
```

## Deleting the `Shoot` cluster

```shell
./hack/usage/delete shoot local garden-local
```

## Tear down the Gardener environment

```shell
make tear-down-local-env
make kind-down
```

## Further reading

This setup makes use of the local provider extension. You can read more about it in [this document](../extensions/provider-local.md).
