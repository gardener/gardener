# Local Provider Extension

The "local provider" extension is used to allow the usage of seed and shoot clusters which run entirely locally without any real infrastructure or cloud provider involved.
It implements Gardener's extension contract ([GEP-1](../proposals/01-extensibility.md)) and thus comprises several controllers and webhooks acting on resources in seed and shoot clusters.

The code is maintained in [`pkg/provider-local`](../../pkg/provider-local).

## Motivation

The motivation for maintaining such extension is the following:

- 🛡 Output Qualification: Run fast and cost-efficient end-to-end tests, locally and in CI systems (increased confidence ⛑ before merging pull requests)
- ⚙️ Development Experience: Develop Gardener entirely on a local machine without any external resources involved (improved costs 💰 and productivity 🚀)
- 🤝 Open Source: Quick and easy setup for a first evaluation of Gardener and a good basis for first contributions

## Current Limitations

The following enlists the current limitations of the implementation.
Please note that all of them are not technical limitations/blockers, but simply advanced scenarios that we haven't had invested yet into.

1. No load balancers for Shoot clusters.

   _We have not yet developed a `cloud-controller-manager` which could reconcile load balancer `Service`s in the shoot cluster._

1. In case a seed cluster with multiple availability zones, i.e. multiple entries in `.spec.provider.zones`, is used in conjunction with a single-zone shoot control plane, i.e. a shoot cluster without `.spec.controlPlane.highAvailability` or with `.spec.controlPlane.highAvailability.failureTolerance.type` set to `node`, the local address of the API server endpoint needs to be determined manually or via the in-cluster `coredns`.

   _As the different istio ingress gateway loadbalancers have individual external IP addresses, single-zone shoot control planes can end up in a random availability zone. Having the local host use the `coredns` in the cluster as name resolver would form a name resolution cycle. The tests mitigate the issue by adapting the DNS configuration inside the affected test._

## `ManagedSeed`s

It is possible to deploy [`ManagedSeed`s](../operations/managed_seed.md) with `provider-local` by first creating a [`Shoot` in the `garden` namespace](../../example/provider-local/managedseeds/shoot-managedseed.yaml) and then creating a referencing [`ManagedSeed` object](../../example/provider-local/managedseeds/managedseed.yaml).

> Please note that this is only supported by the [`Skaffold`-based setup](../deployment/getting_started_locally.md).

The corresponding e2e test can be run via:

```bash
./hack/test-e2e-local.sh --label-filter "ManagedSeed"
```

### Implementation Details

The images locally built by `Skaffold` for the Gardener components which are deployed to this shoot cluster are managed by a container registry in the `registry` namespace in the kind cluster.
`provider-local` configures this registry as mirror for the shoot by mutating the `OperatingSystemConfig` and using the [default contract for extending the `containerd` configuration](../usage/advanced/custom-containerd-config.md).

In order to bootstrap a seed cluster, the `gardenlet` deploys `PersistentVolumeClaim`s and `Service`s of type `LoadBalancer`.
While storage is supported in shoot clusters by using the [`local-path-provisioner`](https://github.com/rancher/local-path-provisioner), load balancers are not supported yet.
However, `provider-local` runs a `Service` controller which specifically reconciles the seed-related `Service`s of type `LoadBalancer`.
This way, they get an IP and `gardenlet` can finish its bootstrapping process.
Note that these IPs are not reachable, however for the sake of developing `ManagedSeed`s this is sufficient for now.

Also, please note that the `provider-local` extension only gets deployed because of the `Always` deployment policy in its corresponding `ControllerRegistration` and because the DNS provider type of the seed is set to `local`.

## Implementation Details

This section contains information about how the respective controllers and webhooks in `provider-local` are implemented and what their purpose is.

### Bootstrapping

The Helm chart of the `provider-local` extension defined in its [`ControllerDeployment`](controllerregistration.md) contains a special deployment for a [CoreDNS](https://coredns.io/) instance in a `gardener-extension-provider-local-coredns` namespace in the seed cluster.

This CoreDNS instance is responsible for enabling the components running in the shoot clusters to be able to resolve the DNS names when they communicate with their `kube-apiserver`s.

It contains a static configuration to resolve the DNS names based on `local.gardener.cloud` to `istio-ingressgateway.istio-ingress.svc`.

### Controllers

There are controllers for all resources in the `extensions.gardener.cloud/v1alpha1` API group except for `BackupBucket` and `BackupEntry`s.

#### `ControlPlane`

This controller is deploying the [local-path-provisioner](https://github.com/rancher/local-path-provisioner) as well as a related `StorageClass` in order to support `PersistentVolumeClaim`s in the local shoot cluster.
Additionally, it creates a few (currently unused) dummy secrets (CA, server and client certificate, basic auth credentials) for the sake of testing the secrets manager integration in the extensions library.

#### `DNSRecord`

The controller adapts the cluster internal DNS configuration by extending the `coredns` configuration for every observed `DNSRecord`. It will add two corresponding entries in the custom DNS configuration per shoot cluster:

```text
data:
  api.local.local.external.local.gardener.cloud.override: |
    rewrite stop name regex api.local.local.external.local.gardener.cloud istio-ingressgateway.istio-ingress.svc.cluster.local answer auto
  api.local.local.internal.local.gardener.cloud.override: |
    rewrite stop name regex api.local.local.internal.local.gardener.cloud istio-ingressgateway.istio-ingress.svc.cluster.local answer auto
```

#### `Infrastructure`

This controller generates a `NetworkPolicy` which allows the control plane pods (like `kube-apiserver`) to communicate with the worker machine pods (see [`Worker` section](#worker)).

#### `Network`

This controller is not implemented anymore. In the initial version of `provider-local`, there was a `Network` controller deploying [kindnetd](https://github.com/kubernetes-sigs/kind/blob/main/images/kindnetd/README.md) (see [release v1.44.1](https://github.com/gardener/gardener/tree/v1.44.1/pkg/provider-local/controller/network)).
However, we decided to drop it because this setup prevented us from using `NetworkPolicy`s (kindnetd does not ship a `NetworkPolicy` controller).
In addition, we had issues with shoot clusters having more than one node (hence, we couldn't support rolling updates, see [PR #5666](https://github.com/gardener/gardener/pull/5666/commits/491b3cd16e40e5c20ef02367fda93a34ff9465eb)).

#### `OperatingSystemConfig`

This controller renders a simple cloud-init template which can later be executed by the shoot worker nodes.

The shoot worker nodes are `Pod`s with a container based on the `kindest/node` image. This is maintained in the [gardener/machine-controller-manager-provider-local repository](https://github.com/gardener/machine-controller-manager-provider-local/tree/master/node) and has a special `run-userdata` systemd service which executes the cloud-init generated earlier by the `OperatingSystemConfig` controller.

#### `Worker`

This controller leverages the standard [generic `Worker` actuator](../../extensions/pkg/controller/worker/genericactuator) in order to deploy the [`machine-controller-manager`](https://github.com/gardener/machine-controller-manager) as well as the [`machine-controller-manager-provider-local`](https://github.com/gardener/machine-controller-manager-provider-local).

Additionally, it generates the [`MachineClass`es](https://github.com/gardener/machine-controller-manager-provider-local/blob/master/kubernetes/machine-class.yaml) and the `MachineDeployment`s based on the specification of the `Worker` resources.

#### `Ingress`

The gardenlet creates a wildcard DNS record for the Seed's ingress domain pointing to the `nginx-ingress-controller`'s LoadBalancer.
This domain is commonly used by all `Ingress` objects created in the Seed for Seed and Shoot components.
As provider-local implements the `DNSRecord` extension API (see the [`DNSRecord`section](#dnsrecord)), this controller reconciles all `Ingress`s and creates `DNSRecord`s of type `local` for each host included in `spec.rules`.
This only happens for shoot namespaces (`gardener.cloud/role=shoot` label) to make `Ingress` domains resolvable on the machine pods.

#### `Service`

This controller reconciles `Services` of type `LoadBalancer` in the local `Seed` cluster.
Since the local Kubernetes clusters used as Seed clusters typically don't support such services, this controller sets the `.status.ingress.loadBalancer.ip[0]` to the IP of the host.
It makes important LoadBalancer Services (e.g. `istio-ingress/istio-ingressgateway` and `garden/nginx-ingress-controller`) available to the host by setting `spec.ports[].nodePort` to well-known ports that are mapped to `hostPorts` in the kind cluster configuration.

`istio-ingress/istio-ingressgateway` is set to be exposed on `nodePort` `30433` by this controller.

In case the seed has multiple availability zones (`.spec.provider.zones`) and it uses SNI, the different zone-specific `istio-ingressgateway` loadbalancers are exposed via different IP addresses. Per default, IP addresses `172.18.255.10`, `172.18.255.11`, and `172.18.255.12` are used for the zones `0`, `1`, and `2` respectively.

#### ETCD Backups
This controller reconciles the `BackupBucket` and `BackupEntry` of the shoot allowing the `etcd-backup-restore` to create and copy backups using the `local` provider functionality. The backups are stored on the host file system. This is achieved by mounting that directory to the `etcd-backup-restore` container.

#### Extension Seed
This controller reconciles `Extensions` of type `local-ext-seed`. It creates a single `serviceaccount` named `local-ext-seed` in the shoot's namespace in the seed. The extension is reconciled before the `kube-apiserver`. More on extension lifecycle strategies can be read in [Registering Extension Controllers](controllerregistration.md#extension-lifecycle).

#### Extension Shoot
This controller reconciles `Extensions` of type `local-ext-shoot`. It creates a single `serviceaccount` named `local-ext-shoot` in the `kube-system` namespace of the shoot. The extension is reconciled after the `kube-apiserver`. More on extension lifecycle strategies can be read [Registering Extension Controllers](controllerregistration.md#extension-lifecycle).

#### Extension Shoot After Worker
This controller reconciles `Extensions` of type `local-ext-shoot-after-worker`. It creates a `deployment` named `local-ext-shoot-after-worker` in the `kube-system` namespace of the shoot. The extension is reconciled after the workers and waits until the deployment is ready. More on extension lifecycle strategies can be read [Registering Extension Controllers](controllerregistration.md#extension-lifecycle).

#### Health Checks

The health check controller leverages the [health check library](healthcheck-library.md) in order to:

- check the health of the `ManagedResource/extension-controlplane-shoot-webhooks` and populate the `SystemComponentsHealthy` condition in the `ControlPlane` resource.
- check the health of the `ManagedResource/extension-networking-local` and populate the `SystemComponentsHealthy` condition in the `Network` resource.
- check the health of the `ManagedResource/extension-worker-mcm-shoot` and populate the `SystemComponentsHealthy` condition in the `Worker` resource.
- check the health of the `Deployment/machine-controller-manager` and populate the `ControlPlaneHealthy` condition in the `Worker` resource.
- check the health of the `Node`s and populate the `EveryNodeReady` condition in the `Worker` resource.

### Webhooks

#### Control Plane

This webhook reacts on the `OperatingSystemConfig` containing the configuration of the kubelet and sets the `failSwapOn` to `false` (independent of what is configured in the `Shoot` spec) ([ref](https://github.com/kubernetes-sigs/kind/blob/b6bc112522651d98c81823df56b7afa511459a3b/site/content/docs/design/node-image.md#design)).

#### DNS Config

This webhook reacts on events for the `dependency-watchdog-probe` `Deployment`, the `blackbox-exporter` `Deployment`, as well as on events for `Pod`s created when the `machine-controller-manager` reconciles `Machine`s.
All these pods need to be able to resolve the DNS names for shoot clusters.
It sets the `.spec.dnsPolicy=None` and `.spec.dnsConfig.nameServers` to the cluster IP of the `coredns` `Service` created in the `gardener-extension-provider-local-coredns` namespaces so that these pods can resolve the DNS records for shoot clusters (see the [Bootstrapping section](#bootstrapping) for more details).

#### Machine Controller Manager

This webhook mutates the global `ClusterRole` related to `machine-controller-manager` and injects permissions for `Service` resources.
The `machine-controller-manager-provider-local` deploys `Pod`s for each `Machine` (while real infrastructure provider obviously deploy VMs, so no Kubernetes resources directly).
It also deploys a `Service` for these machine pods, and in order to do so, the `ClusterRole` must allow the needed permissions for `Service` resources.

#### Node

This webhook reacts on updates to `nodes/status` in both seed and shoot clusters and sets the `.status.{allocatable,capacity}.cpu="100"` and `.status.{allocatable,capacity}.memory="100Gi"` fields.

Background: Typically, the `.status.{capacity,allocatable}` values are determined by the resources configured for the Docker daemon (see for example the [docker Quick Start Guide](https://docs.docker.com/desktop/mac/#resources) for Mac).
Since many of the `Pod`s deployed by Gardener have quite high `.spec.resources.requests`, the `Node`s easily get filled up and only a few `Pod`s can be scheduled (even if they barely consume any of their reserved resources).
In order to improve the user experience, on startup/leader election the provider-local extension submits an empty patch which triggers the "node webhook" (see the below section) for the seed cluster.
The webhook will increase the capacity of the `Node`s to allow all `Pod`s to be scheduled.
For the shoot clusters, this empty patch trigger is not needed since the `MutatingWebhookConfiguration` is reconciled by the `ControlPlane` controller and exists before the `Node` object gets registered.

#### Shoot

This webhook reacts on the `ConfigMap` used by the `kube-proxy` and sets the `maxPerCore` field to `0` since other values don't work well in conjunction with the `kindest/node` image which is used as base for the shoot worker machine pods ([ref](https://github.com/kubernetes-sigs/kind/blob/fa7d86470f4c0e924fc4c2e767ec8491c45f4304/pkg/cluster/internal/kubeadm/config.go#L283-L285)).

### DNS Configuration for Multi-Zonal Seeds

In case a seed cluster has multiple availability zones as specified in `.spec.provider.zones`, multiple istio ingress gateways are deployed, one per availability zone in addition to the default deployment. The result is that single-zone shoot control planes, i.e. shoot clusters with `.spec.controlPlane.highAvailability` set or with `.spec.controlPlane.highAvailability.failureTolerance.type` set to `node`, may be exposed via any of the zone-specific istio ingress gateways. Previously, the endpoints were statically mapped via `/etc/hosts`. Unfortunately, this is no longer possible due to the aforementioned dynamic in the endpoint selection.

For multi-zonal seed clusters, there is an additional configuration following `coredns`'s [view plugin](https://github.com/coredns/coredns/tree/master/plugin/view) mapping the external IP addresses of the zone-specific loadbalancers to the corresponding internal istio ingress gateway domain names. This configuration is only in place for requests from outside of the seed cluster. Those requests are currently being identified by the protocol. UDP requests are interpreted as originating from within the seed cluster while TCP requests are assumed to come from outside the cluster via the docker hostport mapping.

The corresponding test sets the DNS configuration accordingly so that the name resolution during the test use `coredns` in the cluster.

### machine-controller-manager-provider-local

Out of tree (controller-based) implementation for `local` as a new provider.
The local out-of-tree provider implements the interface defined at [MCM OOT driver](https://github.com/gardener/machine-controller-manager/blob/master/pkg/util/provider/driver/driver.go).

#### Fundamental Design Principles

Following are the basic principles kept in mind while developing the external plugin.

- Communication between this Machine Controller (MC) and Machine Controller Manager (MCM) is achieved using the Kubernetes native declarative approach.
- Machine Controller (MC) behaves as the controller used to interact with the `local` provider and manage the VMs corresponding to the machine objects.
- Machine Controller Manager (MCM) deals with higher level objects such as machine-set and machine-deployment objects.

## Future Work

Future work could mostly focus on resolving the above listed [limitations](#limitations), i.e.:

- Implement a `cloud-controller-manager` and deploy it via the [`ControlPlane` controller](#controlplane).
- Properly implement `.spec.machineTypes` in the `CloudProfile`s (i.e., configure `.spec.resources` properly for the created shoot worker machine pods).
