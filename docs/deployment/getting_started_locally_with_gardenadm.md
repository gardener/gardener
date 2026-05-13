# Deploying Self-Hosted Shoot Clusters Locally

> [!CAUTION]
> The `gardenadm` tool is currently under development and considered highly experimental.
> Do not use it in production environments.
> Read more about it in [GEP-0028](https://github.com/gardener/enhancements/tree/main/geps/0028-self-hosted-shoot-clusters).

This document walks you through deploying Self-Hosted Shoot Clusters using `gardenadm` on your local machine.
This setup can be used for trying out and developing `gardenadm` locally without additional infrastructure.
The setup is also used for running e2e tests for `gardenadm` in CI ([Prow](https://prow.gardener.cloud)).

If you encounter difficulties, please open an issue so that we can make this process easier.

## Overview

`gardenadm` is a command line tool for bootstrapping Kubernetes clusters called "Self-Hosted Shoot Clusters". Read the [`gardenadm` documentation](../concepts/gardenadm.md) for more details on its concepts.

The local setup supports both the ["unmanaged infrastructure" and the "managed infrastructure" scenarios](../concepts/gardenadm.md#scenarios).

In the **unmanaged infrastructure** scenario, there is no programmable infrastructure available (the "bare metal" or "edge" use-case).
Machines must be prepared upfront, and network setup as well as machine management are out of scope.
In this local setup, we simulate existing machines by running Docker containers directly via [Docker Compose](https://docs.docker.com/compose/).

In the **managed infrastructure** scenario, programmable infrastructure is available and Gardener leverages [`provider-local`](../extensions/provider-local.md) and [`machine-controller-manager`](https://github.com/gardener/machine-controller-manager) to manage the network setup and machines.
In this local setup, we start a [KinD](https://kind.sigs.k8s.io/) cluster that acts as the programmable infrastructure hosting the machines.

Based on [Skaffold](https://skaffold.dev/), the container images for all required components will be built and deployed via Docker or into the cluster.
This also includes the `gardenadm` CLI, which is installed on the machine containers by pulling the container image and extracting the binary.

## Prerequisites

- Make sure that you have followed the [Local Setup guide](../development/local_setup.md) up until the [Get the sources](../development/local_setup.md#get-the-sources) step.
- Make sure your Docker daemon is up-to-date, up and running and has enough resources (at least `8` CPUs and `8Gi` memory; see [here](https://docs.docker.com/desktop/mac/#resources) how to configure the resources for Docker for Mac).
  > Additionally, please configure at least `120Gi` of disk size for the Docker daemon.

> [!TIP]
> You can clean up unused data with `docker system df` and `docker system prune -a`.

## "Unmanaged Infrastructure" Scenario

Use the following command to prepare the `gardenadm` unmanaged infrastructure scenario:

```shell
make gind-up # Gardener-in-Docker
```

This will first build the needed images, deploy 4 machine containers using the [`gardener-extension-provider-local/node` image](../../pkg/provider-local/machine-provider/node), install the `gardenadm` binary on all of them, and copy the needed manifests to the `/gardenadm/resources` directory:

```shell
$ docker ps | grep gind-machine
CONTAINER ID   IMAGE            COMMAND                  CREATED          STATUS          PORTS   NAMES
08cd7c006fc1   gind-machine-0   "/init-machine-state…"   50 seconds ago   Up 48 seconds           gind-machine-0
4b316de9a8c5   gind-machine-1   "/init-machine-state…"   50 seconds ago   Up 48 seconds           gind-machine-1
dcdb62e429ef   gind-machine-2   "/init-machine-state…"   50 seconds ago   Up 48 seconds           gind-machine-2
c1b74a741416   gind-machine-3   "/init-machine-state…"   50 seconds ago   Up 48 seconds           gind-machine-3
```

Afterward, it automatically runs `gardenadm init` on `gind-machine-0` to bootstrap the first control plane node.
This usually takes a couple of minutes, but eventually you should see output like this:

```shell
Your Shoot cluster control-plane has initialized successfully!
...
```

`make gind-up` supports a `SCENARIO` variable that controls how far the setup proceeds (e.g., `make gind-up SCENARIO=default`):

| `SCENARIO` | Description                                                                                                                                                |
|------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `machines` | Only starts the machine containers and installs `gardenadm`, but doesn't run it.                                                                           |
| `default`  | Like `machines`, but also runs `gardenadm init` and exports the kubeconfig for the self-hosted shoot. This is the default when no `SCENARIO` is specified. |
| `join`     | Like `default`, but also runs `gardenadm join` on `gind-machine-1` to join it as a worker node.                                                            |
| `connect`  | Like `join`, but also deploys Gardener into the self-hosted shoot and runs `gardenadm connect` to deploy gardenlet which registers the `Shoot`.            |
| `full`     | Like `connect`, but also registers the self-hosted shoot as a seed via a `ManagedSeed`, enabling it to host shoot clusters.                                |

> [!TIP]
> You can pass `FAST=true` to skip switching etcd management to etcd-druid and keep `gardener-resource-manager` and extensions in the host network.
> This speeds up `gardenadm init` significantly:
> ```shell
> make gind-up FAST=true
> ```

### Inspecting the Gardener Configuration (`Shoot`, `CloudProfile`, etc.)

If you would like to inspect the resources used to bring up this self-hosted shoot cluster, you can exec into the `gind-machine-0` container:

```shell
$ docker exec -ti gind-machine-0 bash
root@gind-machine-0:/# gardenadm -h
gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.
...

root@gind-machine-0:/# cat /gardenadm/resources/manifests.yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
...
```

### Connecting to the Self-Hosted Shoot Cluster

You can either exec into the machine container and access the cluster from there, or you access it directly from your host machine.

#### Machine Container Access

The machine container's shell environment is configured for easily connecting to the self-hosted shoot cluster.
Just exec into the machine container via the `docker` CLI and run `bash`:

```shell
$ docker exec -ti gind-machine-0 bash
root@gind-machine-0:/# kubectl get node
NAME             STATUS   ROLES           AGE     VERSION
gind-machine-0   Ready    control-plane   7m15s   v1.35.0
```

#### Host Machine Access

A kubeconfig is automatically exported by `make gind-up` to `dev-setup/kubeconfigs/self-hosted-shoot/kubeconfig`.
You can directly use it from there:

```shell
$ export KUBECONFIG=dev-setup/kubeconfigs/self-hosted-shoot/kubeconfig
$ kubectl get no
NAME             STATUS   ROLES           AGE     VERSION
gind-machine-0   Ready    control-plane   7m15s   v1.35.0
```

> [!TIP]
> This works by running an Envoy container next to the machine containers that binds to an IP previously added to the host's loopback device.
> It forwards received traffic to the control plane machines:
> 
> ```shell
> $ docker ps | grep gind-apiserver-lb
> CONTAINER ID   IMAGE                      COMMAND                  CREATED          STATUS          PORTS                         NAMES
> fd75b2c4612d   envoyproxy/envoy:v1.37.0   "/docker-entrypoint.…"   15 minutes ago   Up 15 minutes   172.18.255.123:443->443/tcp   gind-apiserver-lb
> 
> $ ip a
> 1: lo0: <UP,LOOPBACK,RUNNING,MULTICAST> mtu 16384 status UNKNOWN
>     link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
>     inet 127.0.0.1/8
>     inet6 ::1/128
>     ...
>     inet 172.18.255.123/16
> ```

### Joining a Worker Node

> [!TIP]
> This step is automated when using `make gind-up SCENARIO=join` or `make gind-up SCENARIO=connect`.

If you would like to join a worker node to the cluster manually, generate a bootstrap token and the corresponding `gardenadm join` command on `gind-machine-0` (the control plane node).
Then exec into the `gind-machine-1` container to run the command:

```shell
root@gind-machine-0:/# gardenadm token create --print-join-command
# now copy the output, terminate the exec session and start a new one for machine-1

$ docker exec -ti gind-machine-1 bash
# paste the copied 'gardenadm join' command here and execute it
root@gind-machine-1:/# gardenadm join ...
...
Your node has successfully joined the cluster as a worker!
...
```

> [!NOTE]
> Accessing the shoot cluster is only possible from within control plane machine - worker machines (like the just joined `gind-machine-1`) are not prepared accordingly.

Using the kubeconfig as described in [this section](#connecting-to-the-self-hosted-shoot-cluster), you should now be able to see the new node in the cluster:

```shell
$ kubectl get no
NAME        STATUS   ROLES    AGE   VERSION
NAME             STATUS   ROLES           AGE     VERSION
gind-machine-0   Ready    control-plane   7m15s   v1.35.0
gind-machine-1   Ready    worker          8m48s   v1.35.0
```

## "Managed Infrastructure" Scenario

### Setting Up the KinD Cluster

```shell
make kind-up
```

All following steps assume that you are using the kubeconfig for this KinD cluster:

```shell
export KUBECONFIG=$PWD/dev-setup/kubeconfigs/runtime/kubeconfig
```

Use the following command to prepare the `gardenadm` managed infrastructure scenario:

```shell
make gardenadm-up SCENARIO=managed-infra
```

This will first build the needed images and then render the needed manifests for `gardenadm bootstrap` to the [`./dev-setup/gardenadm/resources/generated/managed-infra`](../../dev-setup/gardenadm/resources/generated/managed-infra) directory.

### Bootstrapping the Self-Hosted Shoot Cluster

Use `go run` to execute `gardenadm` commands on your machine:

```shell
$ export IMAGEVECTOR_OVERWRITE=$PWD/dev-setup/gardenadm/resources/generated/.imagevector-overwrite.yaml
$ go run ./cmd/gardenadm bootstrap -d ./dev-setup/gardenadm/resources/generated/managed-infra
...
[shoot--garden--root-control-plane-58ffc-2l6s7] Your Shoot cluster control-plane has initialized successfully!
...
```

### Connecting to the Self-Hosted Shoot Cluster

`gardenadm init` stores the kubeconfig of the self-hosted shoot cluster in the `/etc/kubernetes/admin.conf` file on the control plane machine.
To connect to the self-hosted shoot cluster, set the `KUBECONFIG` environment variable and execute `kubectl` within a `bash` shell in the machine pod:

```shell
$ machine="$(kubectl -n shoot--garden--root get po -l app=machine -oname | head -1 | cut -d/ -f2)"
$ kubectl -n shoot--garden--root exec -it $machine -- bash
root@machine-shoot--garden--root-control-plane-58ffc-2l6s7:/# export KUBECONFIG=/etc/kubernetes/admin.conf
root@machine-shoot--garden--root-control-plane-58ffc-2l6s7:/# kubectl get node
NAME                                                    STATUS   ROLES    AGE     VERSION
machine-shoot--garden--root-control-plane-58ffc-2l6s7   Ready    <none>   4m11s   v1.35.0
```

`gardenadm bootstrap` copies the kubeconfig from the control plane machine to the bootstrap cluster.
You can also copy the kubeconfig to your local machine and use a port-forward to connect to the cluster's API server:

```shell
$ kubectl get secret -n shoot--garden--root kubeconfig -o jsonpath='{.data.kubeconfig}' | base64 --decode | sed 's/api.root.garden.external.local.gardener.cloud/localhost:6443/' > /tmp/shoot--garden--root.conf
$ machine="$(kubectl -n shoot--garden--root get po -l app=machine -oname | head -1 | cut -d/ -f2)"
$ kubectl -n shoot--garden--root port-forward pod/$machine 6443:443

# in a new terminal
$ export KUBECONFIG=/tmp/shoot--garden--root.conf
$ kubectl get no
NAME                                                    STATUS   ROLES    AGE     VERSION
machine-shoot--garden--root-control-plane-58ffc-2l6s7   Ready    <none>   4m11s   v1.35.0
```

### Tearing Down the KinD Cluster

When you are done, you can delete the setup by running

```shell
make kind-down
```

## Connecting the Self-Hosted Shoot Cluster to Gardener

> [!TIP]
> For the unmanaged infrastructure scenario, this step is automated when using `make gind-up SCENARIO=connect`.

After you have successfully bootstrapped a self-hosted shoot cluster (either via the [unmanaged infrastructure](#unmanaged-infrastructure-scenario) or the [managed infrastructure](#managed-infrastructure-scenario) scenario), you can connect it to an existing Gardener system.
For this, you need to deploy Gardener to your self-hosted shoot cluster.
In order to deploy it, you can run

```shell
make gardenadm-up SCENARIO=connect
```

This will deploy [`gardener-operator`](../concepts/operator.md) and create a `Garden` resource (which will then be reconciled and results in a full Gardener deployment) inside the self-hosted shoot cluster.
Find all information about it [here](getting_started_locally.md).

> [!NOTE]
> There is an alternative way of deploying Gardener outside the self-hosted shoot but inside a KinD cluster in the
> `garden` namespace.
>
> `make kind-up`
> `make gardenadm-up SCENARIO=connect-kind`
>
> The following steps from above are the same.

In all cases, the kubeconfig for the garden cluster gets exported to `/dev-setup/kubeconfigs/virtual-garden/kubeconfig`.

Note, that in this setup, no `Seed` will be registered in the Gardener - it's just a plain garden cluster without the ability to create regular shoot clusters.

Once above command is finished, you can generate a bootstrap token using `gardenadm` to connect the shoot cluster to this Gardener instance.
For this, you must have installed the `gardenadm` binary locally. You can build it via:

```shell
make gardenadm
```

This will install it to `./bin/gardenadm`, from where you can call it.

Now you can generate the bootstrap token and the full `gardenadm connect` command like this:

```shell
$ KUBECONFIG=./dev-setup/kubeconfigs/virtual-garden/kubeconfig ./bin/gardenadm token create --print-connect-command --shoot-namespace=garden --shoot-name=root
# This will output a command similar to:
gardenadm connect --bootstrap-token ... --ca-certificate ... https://api.virtual-garden.local.gardener.cloud
```

Copy the full output, exec once again into one of the control-plane machines of your self-hosted shoot cluster, and paste and run the generated `gardenadm connect` command there:

```shell
root@gind-machine-0:/# gardenadm connect --bootstrap-token ... --ca-certificate ... https://api.virtual-garden.local.gardener.cloud
2025-11-10T08:12:32.287Z	INFO	Using resources from directory	{"configDir": "/gardenadm/resources/"}
2025-11-10T08:12:32.334Z	INFO	Initializing gardenadm botanist with fake client set	{"cloudProfile": {"apiVersion": "core.gardener.cloud/v1beta1", "kind": "CloudProfile", "name": "local"}, "project": {"apiVersion": "core.gardener.cloud/v1beta1", "kind": "Project", "name": "garden"}, "shoot": {"apiVersion": "core.gardener.cloud/v1beta1", "kind": "Shoot", "namespace": "garden", "name": "root"}}
2025-11-10T08:12:32.345Z	INFO	Starting	{"flow": "connect"}
...
2025-11-10T08:13:04.571Z	INFO	Succeeded	{"flow": "connect", "task": "Waiting until gardenlet is ready"}
2025-11-10T08:13:04.571Z	INFO	Finished	{"flow": "connect"}
```

Once this is done, you can observe that there is now a `gardenlet` running in the self-hosted shoot cluster, which connects it to the Gardener instance:

```shell
root@gind-machine-0:/# kubectl get pods -n kube-system -l app=gardener,role=gardenlet
gardenlet-6cbcb676f5-prh8f                           1/1     Running   0             40m
gardenlet-6cbcb676f5-wwn8w                           1/1     Running   0             40m
````

You can also observe that the self-hosted shoot cluster is now registered as a shoot cluster in Gardener:

```shell
kubectl --kubeconfig=./dev-setup/kubeconfigs/virtual-garden/kubeconfig get shoots -A
NAMESPACE   NAME   CLOUDPROFILE   PROVIDER   REGION   K8S VERSION   HIBERNATION   LAST OPERATION   STATUS    AGE
garden      root   local          local      local    1.35.0        Awake         <pending>        healthy   42m
```

## Promoting the Self-Hosted Shoot to a Seed

> [!TIP]
> For the unmanaged infrastructure scenario, this step is automated when using `make gind-up SCENARIO=full`.

After the self-hosted shoot has been connected to Gardener (see above), you can register it as a seed by creating a `ManagedSeed` resource.
This enables the self-hosted shoot to host other shoot clusters.

```shell
make seed-up KUBECONFIG=./dev-setup/kubeconfigs/self-hosted-shoot/kubeconfig
```

This deploys a seed gardenlet via a `ManagedSeed` into the self-hosted shoot.

> ![NOTE]
> The following steps assume that you are using the kubeconfig that points to the virtual garden cluster: `export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig`.

You can wait for the `Seed` to be ready by running:

```bash
./hack/usage/wait-for.sh seed root GardenletReady SeedSystemComponentsHealthy ExtensionsReady
```

Alternatively, you can run `kubectl get seed root` and wait for the `STATUS` to indicate readiness:

```bash
NAME   STATUS   LAST OPERATION               PROVIDER   REGION   AGE   VERSION      K8S VERSION
root   Ready    Reconcile Succeeded (100%)   local      local    37m   vX.Y.Z-dev   v1.35.0
```

Once the `Seed` is ready, you can create shoot clusters on top of it.

### Creating a (Hosted) `Shoot` Cluster

> ![NOTE]
> The following steps assume that you are using the kubeconfig that points to the virtual garden cluster: `export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig`.

In order to create a first (hosted) shoot cluster, just run:

```bash
kubectl apply -f example/provider-local/shoot.yaml
```

You can wait for the `Shoot` to be ready by running:

```bash
NAMESPACE=garden-local ./hack/usage/wait-for.sh shoot local APIServerAvailable ControlPlaneHealthy ObservabilityComponentsHealthy EveryNodeReady SystemComponentsHealthy
```

Alternatively, you can run `kubectl -n garden-local get shoot local` and wait for the `LAST OPERATION` to reach `100%`:

```bash
NAME    CLOUDPROFILE   PROVIDER   REGION   K8S VERSION   HIBERNATION   LAST OPERATION            STATUS    AGE
local   local          local      local    1.35.0        Awake         Create Processing (43%)   healthy   94s
```

If you don't need any worker pools, you can create a workerless `Shoot` by running:

```bash
kubectl apply -f example/provider-local/shoot-workerless.yaml
```

#### Accessing the `Shoot` Cluster

To access the `Shoot`, you can acquire a `kubeconfig` by using the [`shoots/adminkubeconfig` subresource](../usage/shoot/shoot_access.md#shootsadminkubeconfig-subresource).

For convenience a [helper script](../../hack/usage/generate-kubeconfig.sh) is provided in the `hack` directory.
By default, the script will generate an admin kubeconfig for a `Shoot` named "local" in the `garden-local` namespace valid for one hour.

```bash
./hack/usage/generate-kubeconfig.sh > admin-kubeconf.yaml
```

To generate a viewer kubeconfig instead of an admin kubeconfig, use the `--viewer` flag:

```bash
./hack/usage/generate-kubeconfig.sh --viewer > viewer-kubeconf.yaml
```

> [!NOTE]
> Keep in mind that using a VPN on your local machine could cause problems with the setup, and the shoot's kubeconfig could fail with connection issues.
> If you experience connection problems using the shoot's kubeconfig, try disabling the VPN first.

If you want to change the default namespace or shoot name, you can do so by passing different values as arguments.

```bash
./hack/usage/generate-kubeconfig.sh --namespace <namespace> --shoot-name <shootname> > admin-kubeconf.yaml
```

## Running E2E Tests for `gardenadm`

Based on the described setup, you can execute the e2e test suite for `gardenadm`:

```shell
make gind-up
make test-e2e-local-gardenadm-unmanaged-infra-initjoin
make gardenadm-up SCENARIO=connect
make test-e2e-local-gardenadm-unmanaged-infra-connect
make seed-up KUBECONFIG=./dev-setup/kubeconfigs/self-hosted-shoot/kubeconfig
make test-e2e-local-gardenadm-unmanaged-infra-seed

# or
make gardenadm-up SCENARIO=managed-infra
make test-e2e-local-gardenadm-managed-infra
```
