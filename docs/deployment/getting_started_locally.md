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

## Setting Up the KinD Cluster (Garden and Seed)

```bash
make kind-up
```

> If you want to setup an IPv6 KinD cluster, use `make kind-up IPFAMILY=ipv6` instead.

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./example/gardener-local/kind/local/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config`, since you will need to target this KinD cluster multiple times.
> Alternatively, make sure to set your `KUBECONFIG` environment variable to `./example/gardener-local/kind/local/kubeconfig` for all future steps via `export KUBECONFIG=$PWD/example/gardener-local/kind/local/kubeconfig`.

All of the following steps assume that you are using this kubeconfig.

Additionally, this command also deploys a local container registry to the cluster, as well as a few registry mirrors, that are set up as a pull-through cache for all upstream registries Gardener uses by default.
This is done to speed up image pulls across local clusters.
The local registry can be accessed as `localhost:5001` for pushing and pulling.
The storage directories of the registries are mounted to the host machine under `dev/local-registry`.
With this, mirrored images don't have to be pulled again after recreating the cluster.

The command also deploys a default [calico](https://github.com/projectcalico/calico) installation as the cluster's CNI implementation with `NetworkPolicy` support (the default `kindnet` CNI doesn't provide `NetworkPolicy` support).
Furthermore, it deploys the [metrics-server](https://github.com/kubernetes-sigs/metrics-server) in order to support HPA and VPA on the seed cluster.

## Setting Up IPv6 Single-Stack Networking (optional)

First, ensure that your `/etc/hosts` file contains an entry resolving `localhost` to the IPv6 loopback address:

```text
::1 localhost
```

Typically, only `ip6-localhost` is mapped to `::1` on linux machines.
However, we need `localhost` to resolve to both `127.0.0.1` and `::1` so that we can talk to our registry via a single address (`localhost:5001`).

Next, we need to configure NAT for outgoing traffic from the kind network to the internet.
After executing `make kind-up IPFAMILY=ipv6`, execute the following command to set up the corresponding iptables rules:

```bash
ip6tables -t nat -A POSTROUTING -o $(ip route show default | awk '{print $5}') -s fd00:10::/64 -j MASQUERADE
```

## Setting Up Gardener

```bash
make gardener-up
```

> If you want to setup an IPv6 ready Gardener, use `make gardener-up IPFAMILY=ipv6` instead.

This will first build the base images (which might take a bit if you do it for the first time).
Afterwards, the Gardener resources will be deployed into the cluster.

## Developing Gardener

```bash
make gardener-dev
```

This is similar to `make gardener-up` but additionally starts a [skaffold dev loop](https://skaffold.dev/docs/workflows/dev/).
After the initial deployment, skaffold starts watching source files.
Once it has detected changes, press any key to trigger a new build and deployment of the changed components.

Tip: you can set the `SKAFFOLD_MODULE` environment variable to select specific modules of the skaffold configuration (see [`skaffold.yaml`](../../skaffold.yaml)) that skaffold should watch, build, and deploy.
This significantly reduces turnaround times during development.

For example, if you want to develop changes to gardenlet:

```bash
# initial deployment of all components
make gardener-up
# start iterating on gardenlet without deploying other components
make gardener-dev SKAFFOLD_MODULE=gardenlet
```

## Debugging Gardener

```bash
make gardener-debug
```

This is using skaffold debugging features. In the Gardener case, Go debugging using [Delve](https://github.com/go-delve/delve) is the most relevant use case.
Please see the [skaffold debugging documentation](https://skaffold.dev/docs/workflows/debug/) how to setup your IDE accordingly.

`SKAFFOLD_MODULE` environment variable is working the same way as described for [Developing Gardener](#developing-gardener). However, skaffold is not watching for changes when debugging,
because it would like to avoid interrupting your debugging session.

For example, if you want to debug gardenlet:

```bash
# initial deployment of all components
make gardener-up
# start debugging gardenlet without deploying other components
make gardener-debug SKAFFOLD_MODULE=gardenlet
```

In debugging flow, skaffold builds your container images, reconfigures your pods and creates port forwardings for the `Delve` debugging ports to your localhost.
The default port is `56268`. If you debug multiple pods at the same time, the port of the second pod will be forwarded to `56269` and so on.
Please check your console output for the concrete port-forwarding on your machine.

> Note: Resuming or stopping only a single goroutine (Go Issue [25578](https://github.com/golang/go/issues/25578), [31132](https://github.com/golang/go/issues/31132)) is currently not supported, so the action will cause all the goroutines to get activated or paused.
([vscode-go wiki](https://github.com/golang/vscode-go/wiki/debugging#connecting-to-headless-delve-with-target-specified-at-server-start-up))

### Readiness and Leader Election Checks

Standard Kubernetes health checks deserve attention when debugging Gardener using Delve. When Gardener controllers encounter an error condition (such as a missing CRD), they may enter (or remain in) an unready state. Under normal deployment
circumstances with readiness checks enabled, the controllers will be restarted by Kubernetes until the undesirable preconditions clear and the controller can fully start. But this will not happen if the readiness checks are disabled for
debugging purposes.

This means that when a goroutine of gardenlet (or any other gardener-core component you try to debug) is paused on a breakpoint, all the other goroutines (including those responding to health checks) are also paused. Kubernetes controllers
are *not* paused though. For instance, when the whole gardenlet process is paused, it can not renew its lease and can not respond to the liveness and readiness probes. The Kubernetes control plane interprets that controller as failed and
attempts to restart it. That's obviously disruptive to a debugging session with Delve!

To alleviate some of this, Skaffold automatically increases `timeoutSeconds` of liveness and readiness probes to 600, but sometimes this is not enough to resolve an issue -- or is too long for other controllers which depend on responses.
The new developer will need to be initially cautious in their interpretations of these states, but they become second nature over time.

Thus, new developers are advised to set `ENABLE_HEALTH_CHECKS`, both at the command line with the various make environments and with Google Cloud Code IDE plugin. This will cause health checks to be deployed by Helm as they would be in
production. Note this is **_not_** a good long term debugging configuration though.

```bash
make gardener-debug ENABLE_HEALTH_CHECKS=admission,controller,scheduler,gardenlet,operator
```

Once a developer discovers a health check disturbing their debugging workflow, they should remove the check from the `ENABLE_HEALTH_CHECKS` list. The available checks are currently from the set
of `{admission,controller,scheduler, gardenlet, operator}`.

Note that this variable is separate from `SKAFFOLD_MODULES` and may cause some confusion. `SKAFFOLD_MODULES` is defined by Skaffold itself and substitutes for the `--modules` CLI flag. In a Skaffold configuration containing multiple
`Config` elements, the module list specifies which configurations should be loaded. Because many Skaffold configs deploy more than one controller and health check, disabling all healthchecks for a module may be undesirable.

As a developer becomes familiar with the environment, they are unlikely to use this variable at all, preferring to manually take over the responsibilities of the health check facilities and restarting stuck pods as needed.

Similarly, leader election for `gardener-admission-controller`, `gardener-apiserver`, `gardener-controller-manager`, `gardener-scheduler`,`gardenlet` and `operator` are disabled when debugging.

As a developer, you will learn to recognize the same problems in other components which are not deployed by Skaffold, you can temporarily turn off the leader election and disable liveness and readiness probes there too.

### Configuring Skaffold

Different make targets all run the same bare target such as `skaffold run` or `skaffold debug`. The different targets configure different combinations of environment variables to effect the final build and deployment configuration. In turn,
the environment variables activate different Skaffold profiles in the various configurations. This provides a unified configuration palette that can be used in all launch environments (Makefile, CLI and Cloud Code).

The Makefile forms the canonical reference for the required combinations of environment variables. The key configuration variables for a given Skaffold invocation target are printed by make before an invocation. Users who wish to bypass the
Makefile for whatever reason may copy these variables to their environment, such as in the construction of Google Cloud Code or CLI configuration.

### Debugging with an IDE

Google Cloud Code is an IDE plugin that works with Skaffold to launch and synchronize deployments using Skaffold's gRPC facility. It is available for both Jetbrains and Microsoft IDEs. The advantage of using it to launch Skaffold with the
IDE is the various controller directories will be automatically mapped to the correct Delve debugger instances. This can save considerable toil with each development iteration on large deployments such as Gardener. Refer to the 
respective IDE documentation for Cloud Code, making sure to include the correct environment variables for your environment as described in the previous section.

## Creating a `Shoot` Cluster

You can wait for the `Seed` to be ready by running:

```bash
./hack/usage/wait-for.sh seed local GardenletReady SeedSystemComponentsHealthy ExtensionsReady
```

Alternatively, you can run `kubectl get seed local` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local   Ready    local      local    4m42s   vX.Y.Z-dev    v1.25.1
```

In order to create a first shoot cluster, just run:

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
local   local          local      local    1.25.1        Awake         Create Processing (43%)   healthy   94s
```

If you don't need any worker pools, you can create a workerless `Shoot` by running:

```bash
kubectl apply -f example/provider-local/shoot-workerless.yaml
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
127.0.0.1 api.e2e-hib.local.external.local.gardener.cloud
127.0.0.1 api.e2e-hib.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-hib-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-hib-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-unpriv.local.external.local.gardener.cloud
127.0.0.1 api.e2e-unpriv.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-wake-up.local.external.local.gardener.cloud
127.0.0.1 api.e2e-wake-up.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-wake-up-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-wake-up-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-migrate.local.external.local.gardener.cloud
127.0.0.1 api.e2e-migrate.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-migrate-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-migrate-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-rotate.local.external.local.gardener.cloud
127.0.0.1 api.e2e-rotate.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-rotate-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-rotate-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-default.local.external.local.gardener.cloud
127.0.0.1 api.e2e-default.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-default-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-default-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-force-delete.local.external.local.gardener.cloud
127.0.0.1 api.e2e-force-delete.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-fd-hib.local.external.local.gardener.cloud
127.0.0.1 api.e2e-fd-hib.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upd-node.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upd-node.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upd-node-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upd-node-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upgrade.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upgrade.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upgrade-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upgrade-wl.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upg-hib.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upg-hib.local.internal.local.gardener.cloud
127.0.0.1 api.e2e-upg-hib-wl.local.external.local.gardener.cloud
127.0.0.1 api.e2e-upg-hib-wl.local.internal.local.gardener.cloud
EOF
```

To access the `Shoot`, you can acquire a `kubeconfig` by using the [`shoots/adminkubeconfig` subresource](../usage/shoot_access.md#shootsadminkubeconfig-subresource).

## (Optional): Setting Up a Second Seed Cluster

There are cases where you would want to create a second seed cluster in your local setup. For example, if you want to test the [control plane migration](../operations/control_plane_migration.md) feature. The following steps describe how to do that.

If you are on macOS, add a new IP address on your loopback device which will be necessary for the new KinD cluster that you will create. On macOS, the default loopback device is `lo0`.

```bash
sudo ip addr add 127.0.0.2 dev lo0                                     # adding 127.0.0.2 ip to the loopback interface
```

Next, setup the second KinD cluster:

```bash
make kind2-up
```

This command sets up a new KinD cluster named `gardener-local2` and stores its kubeconfig in the `./example/gardener-local/kind/local2/kubeconfig` file.

In order to deploy required resources in the KinD cluster that you just created, run:

```bash
make gardenlet-kind2-up
```

The following steps assume that you are using the kubeconfig that points to the `gardener-local` cluster (first KinD cluster): `export KUBECONFIG=$PWD/example/gardener-local/kind/local/kubeconfig`.

You can wait for the `local2` `Seed` to be ready by running:

```bash
./hack/usage/wait-for.sh seed local2 GardenletReady SeedSystemComponentsHealthy ExtensionsReady
```

Alternatively, you can run `kubectl get seed local2` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local2  Ready    local      local    4m42s   vX.Y.Z-dev    v1.25.1
```

If you want to perform control plane migration, you can follow the steps outlined in [Control Plane Migration](../operations/control_plane_migration.md) to migrate the shoot cluster to the second seed you just created.

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

To access Plutono, Prometheus or other components in a browser, two port forwards are needed:

The port forward from the laptop to the pod:

```shell
k port-forward deployment/remote-local-setup 3000
```

The port forward in the remote-local-setup pod to the respective component:

```shell
k port-forward -n shoot--local--local deployment/plutono 3000
```

## Related Links

- [Local Provider Extension](../extensions/provider-local.md)
