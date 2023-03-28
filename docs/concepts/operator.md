# Gardener Operator

The `gardener-operator` is meant to be responsible for the garden cluster environment.
Without this component, users must deploy ETCD, the Gardener control plane, etc., manually and with separate mechanisms (not maintained in this repository).
This is quite unfortunate since this requires separate tooling, processes, etc.
A lot of production- and enterprise-grade features were built into Gardener for managing the seed and shoot clusters, so it makes sense to re-use them as much as possible also for the garden cluster.

**⚠️ Consider this component highly experimental and DO NOT use it in production.**

## Deployment

There is a [Helm chart](../../charts/gardener/operator) which can be used to deploy the `gardener-operator`.
Once deployed and ready, you can create a `Garden` resource.
Note that there can only be one `Garden` resource per system at a time.

> ℹ️ Similar to seed clusters, garden runtime clusters require a [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), see [this section](#vertical-pod-autoscaler).
> By default, `gardener-operator` deploys the VPA components.
> However, when there already is a VPA available, then set `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=false` in the `Garden` resource.

## Using Garden Runtime Cluster As Seed Cluster

In production scenarios, you probably wouldn't use the Kubernetes cluster running `gardener-operator` and the Gardener control plane (called "runtime cluster") as seed cluster at the same time.
However, such setup is technically possible and might simplify certain situations (e.g., development, evaluation, ...).

If the runtime cluster is a seed cluster at the same time, [`gardenlet`'s `Seed` controller](./gardenlet.md#seed-controller) will not manage the components which were already deployed (and reconciled) by `gardener-operator`.
As of today, this applies to:

- `gardener-resource-manager`
- `vpa-{admission-controller,recommender,updater}`
- `hvpa-controller` (when `HVPA` feature gate is enabled)
- `etcd-druid`

Those components are so-called "seed system components".
As they were already made available by `gardener-operator`, the `gardenlet` just skips them.

> ℹ️ There is no need to configure anything - the `gardenlet` will automatically detect when its seed cluster is the garden runtime cluster at the same time.

⚠️ Note that such setup requires that you upgrade the versions of `gardener-operator` and `gardenlet` in lock-step.
Otherwise, you might experience unexpected behaviour or issues with your seed or shoot clusters.

## `Garden` Resources

Please find an exemplary `Garden` resource [here](../../example/operator/20-garden.yaml).

### Settings For Runtime Cluster

The `Garden` resource offers a few settings that are used to control the behaviour of `gardener-operator` in the runtime cluster.
This section provides an overview over the available settings in `.spec.runtimeCluster.settings`:

#### Load Balancer Services

`gardener-operator` creates a Kubernetes `Service` object of type `LoadBalancer` in the runtime cluster.
It is used for exposing the virtual garden control planes, namely the `virtual-garden-kube-apiserver`.
In most cases, the `cloud-controller-manager` (responsible for managing these load balancers on the respective underlying infrastructure) supports certain customization and settings via annotations.
[This document](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) provides a good overview and many examples.

By setting the `.spec.settings.loadBalancerServices.annotations` field the Gardener administrator can specify a list of annotations which will be injected into the `Service`s of type `LoadBalancer`.

Note that we might switch to exposing the `virtual-garden-kube-apiserver` via Istio in the future (similar to how the `kube-apiservers` of shoot clusters are exposed).
The load balancer service settings might still be relevant, though.

#### Vertical Pod Autoscaler

`gardener-operator` heavily relies on the Kubernetes [`vertical-pod-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
By default, the `Garden` controller deploys the VPA components into the `garden` namespace of the respective runtime cluster.
In case you want to manage the VPA deployment on your own or have a custom one, then you might want to disable the automatic deployment of `gardener-operator`.
Otherwise, you might end up with two VPAs which will cause erratic behaviour.
By setting the `.spec.settings.verticalPodAutoscaler.enabled=false` you can disable the automatic deployment.

⚠️ In any case, there must be a VPA available for your runtime cluster.
Using a runtime cluster without VPA is not supported.

## Credentials Rotation

The credentials rotation works in the same way as it does for `Shoot` resources, i.e. there are `gardener.cloud/operation` annotation values for starting or completing the rotation procedures.

For certificate authorities, `gardener-operator` generates one which is automatically rotated roughly each month (`ca-garden-runtime`) and several CAs which are **NOT** automatically rotated but only on demand.

**🚨 Hence, it is the responsibility of the operator to regularly perform the credentials rotation.**

Please refer to [this document](../usage/shoot_credentials_rotation.md#gardener-provided-credentials) for more details. As of today, `gardener-operator` only creates the following types of credentials (i.e., some sections of the document don't apply for `Garden`s and can be ignored):

- certificate authorities (and related server and client certificates)
- ETCD encryption key
- `ServiceAccount` token signing key

⚠️ Since `kube-controller-manager` is not yet deployed by `gardener-operator`, rotation of static `ServiceAccount` secrets is not supported and must be performed manually after the `Garden` has reached `Prepared` phase before completing the rotation.

⚠️ Rotation of the static kubeconfig (which is enabled unconditionally) is not support for now.
The reason is that it such static kubeconfig will be disabled without configuration option in the near future.
Instead, we'll implement an approach similar to the [`adminkubeconfig` subresource on `Shoot`s](../usage/shoot_access.md#shootsadminkubeconfig-subresource) which can be used to retrieve a temporary kubeconfig for the virtual garden cluster.

## Local Development

The easiest setup is using a local [KinD](https://kind.sigs.k8s.io/) cluster and the [Skaffold](https://skaffold.dev/) based approach to deploy and develop the `gardener-operator`.

### Setting Up the KinD Cluster (runtime cluster)

```shell
make kind-operator-up
```

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./example/gardener-local/kind/operator/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config`, since you will need to target this KinD cluster multiple times.
Alternatively, make sure to set your `KUBECONFIG` environment variable to `./example/gardener-local/kind/operator/kubeconfig` for all future steps via `export KUBECONFIG=example/gardener-local/kind/operator/kubeconfig`.
 
All of the following steps assume that you are using this kubeconfig.


### Setting Up Gardener Operator

```shell
make operator-up
```

This will first build the base images (which might take a bit if you do it for the first time).
Afterwards, the Gardener Operator resources will be deployed into the cluster.

### Developing Gardener Operator (Optional)

```shell
make operator-dev
```

This is similar to `make operator-up` but additionally starts a [skaffold dev loop](https://skaffold.dev/docs/workflows/dev/).
After the initial deployment, skaffold starts watching source files.
Once it has detected changes, press any key to trigger a new build and deployment of the changed components.

### Creating a `Garden`

In order to create a garden, just run:

```shell
kubectl apply -f example/operator/20-garden.yaml
```

You can wait for the `Garden` to be ready by running:

```shell
./hack/usage/wait-for.sh garden garden Reconciled
```

Alternatively, you can run `kubectl get garden` and wait for the `RECONCILED` status to reach `True`:

```shell
NAME     RECONCILED    AGE
garden   Progressing   1s
```

(Optional): Instead of creating above `Garden` resource manually, you could execute the e2e tests by running:

```shell
make test-e2e-local-operator
```

#### Accessing the Virtual Garden Cluster

⚠️ Please note that in this setup, the virtual garden cluster is not accessible by default when you download the kubeconfig and try to communicate with them.
The reason is that your host most probably cannot resolve the DNS names of the clusters.
Hence, if you want to access the virtual garden cluster, you have to run the following command which will extend your `/etc/hosts` file with the required information to make the DNS names resolvable:

```shell
cat <<EOF | sudo tee -a /etc/hosts

# Manually created to access local Gardener virtual garden cluster.
# TODO: Remove this again when the virtual garden cluster access is no longer required.
127.0.0.1 api.virtual-garden.local.gardener.cloud
EOF
```

To access the virtual garden, you can acquire a `kubeconfig` by

```shell
kubectl -n garden get secret -l name=user-kubeconfig -o jsonpath={..data.kubeconfig} | base64 -d > /tmp/virtual-garden-kubeconfig
kubectl --kubeconfig /tmp/virtual-garden-kubeconfig get namespaces
```

### Deleting the `Garden`

```shell
./hack/usage/delete garden garden
```

### Tear Down the Gardener Operator Environment

```shell
make operator-down
make kind-operator-down
```

## Alternative Development Variant

An alternative approach is to start the process locally and manually deploy the `CustomResourceDefinition` for the `Garden` resources into the targeted cluster (potentially remote):

```shell
kubectl create -f example/operator/10-crd-operator.gardener.cloud_gardens.yaml
make start-operator KUBECONFIG=...

# now you can create Garden resources, for example
kubectl create -f example/operator/20-garden.yaml
# alternatively, you can run the e2e test
make test-e2e-local-operator KUBECONFIG=...
```

## Implementation Details

### Controllers

As of today, the `gardener-operator` only has one controller which is now described in more detail.

#### [`Garden` Controller](../../pkg/operator/controller/garden)

The reconciler first generates a general CA certificate which is valid for ~`30d` and auto-rotated when 80% of its lifetime is reached.
Afterwards, it brings up the so-called "garden system components".
The [`gardener-resource-manager`](./resource-manager.md) is deployed first since its `ManagedResource` controller will be used to bring up the remainders.

Other system components are:

- garden system resources ([`PriorityClass`es](../development/priority-classes.md) for the workload resources)
- Vertical Pod Autoscaler (if enabled via `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=true` in the `Garden`)
- HVPA controller (when `HVPA` feature gate is enabled)
- ETCD Druid

As soon as all system components are up, the reconciler deploys the virtual garden cluster.
It comprises out of two ETCDs (one "main" etcd, one "events" etcd) which are managed by ETCD Druid via `druid.gardener.cloud/v1alpha1.Etcd` custom resources.
The whole management works similar to how it works for `Shoot`s, so you can take a look at [this document](etcd.md) for more information in general.

The virtual garden control plane components are:

- `virtual-garden-etcd-main`
- `virtual-garden-etcd-events`
- `virtual-garden-kube-apiserver`

If the `.spec.virtualCluster.controlPlane.highAvailability={}` is set then these components will be deployed in a "highly available" mode.
For ETCD, this means that there will be 3 replicas each.
This works similar like for `Shoot`s (see [this document](../usage/shoot_high_availability.md)) except for the fact that there is no failure tolerance type configurability.
The `gardener-resource-manager`'s [HighAvailabilityConfig webhook](resource-manager.md#high-availability-config) makes sure that all pods with multiple replicas are spread on nodes, and if there are at least two zones in `.spec.runtimeCluster.provider.zones` then they also get spread across availability zones.

> If once set, removing `.spec.virtualCluster.controlPlane.highAvailability` again is not supported.

The `virtual-garden-kube-apiserver` `Deployment` is exposed via a `Service` of type `LoadBalancer` with the same name.
In the future, we might switch to exposing it via Istio, similar to how the `kube-apiservers` of shoot clusters are exposed.

Similar to the `Shoot` API, the version of the virtual garden cluster is controlled via `.spec.virtualCluster.kubernetes.version`.
Likewise, specific configuration for the control plane components can be provided in the same section, e.g. via `.spec.virtualCluster.kubernetes.kubeAPIServer` for the `kube-apiserver`.

For the virtual cluster, it is essential to provide a DNS domain via `.spec.virtualCluster.dns.domain`.
**The respective DNS record is not managed by `gardener-operator` and should be manually created and pointed to the load balancer IP of the `virtual-garden-kube-apiserver` `Service`.**
The DNS domain is used for the `server` in the kubeconfig, and for configuring the `--external-hostname` flag of the API server.

It is also mandatory to provide an IPv4 CIDR for the service network of the virtual cluster via `.spec.virtualCluster.networking.services`.
This range is used by the API server to compute the cluster IPs of `Service`s.

The controller maintains the `Reconciled` condition which indicates the status of an operation.

### Webhooks

As of today, the `gardener-operator` only has one webhook handler which is now described in more detail.

#### Validation

This webhook handler validates `CREATE`/`UPDATE`/`DELETE` operations on `Garden` resources.
Simple validation is performed via [standard CRD validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation).
However, more advanced validation is hard to express via these means and is performed by this webhook handler.

Furthermore, for deletion requests, it is validated that the `Garden` is annotated with a deletion confirmation annotation, namely `confirmation.gardener.cloud/deletion=true`.
Only if this annotation is present it allows the `DELETE` operation to pass.
This prevents users from accidental/undesired deletions.

Another validation is to check that there is only one `Garden` resource at a time.
It prevents creating a second `Garden` when there is already one in the system.

#### Defaulting

This webhook handler mutates the `Garden` resource on `CREATE`/`UPDATE`/`DELETE` operations.
Simple defaulting is performed via [standard CRD defaulting](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#defaulting).
However, more advanced defaulting is hard to express via these means and is performed by this webhook handler.
