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

> ℹ️ Similarly to seed clusters, garden runtime clusters require a [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), see [this section](#vertical-pod-autoscaler).
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
This section provides an overview over the available settings:

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

The credentials rotation works in the same way like it does for `Shoot` resources, i.e. there are `gardener.cloud/operation` annotation values for starting or completing the rotation procedures.

For certificate authorities, `gardener-operator` generates one which is automatically rotated roughly each month (`ca-garden-runtime`) and several CAs which are **NOT** automatically rotated but only on demand.

**🚨 Hence, it is the responsibility of the operator to regularly perform the credentials rotation.**

Please refer to [this document](../usage/shoot_credentials_rotation.md#gardener-provided-credentials) for more details. As of today, `gardener-operator` only creates the following types of credentials (i.e., some sections of the document don't apply for `Garden`s and can be ignored):

- certificate authorities (and related server and client certificates) 

## Local Development

The easiest setup is using a local [KinD](https://kind.sigs.k8s.io/) cluster and the [Skaffold](https://skaffold.dev/) based approach to deploy the `gardener-operator`.

```shell
make kind-operator-up
make operator-up

# now you can create Garden resources, for example
kubectl create -f example/operator/20-garden.yaml
# alternatively, you can run the e2e test
make test-e2e-local-operator

make operator-down
make kind-operator-down
```

Generally, any Kubernetes cluster can be used.
An alternative approach is to start the process locally and manually deploy the `CustomResourceDefinition` for the `Garden` resources into the targeted cluster (potentially remote):

```shell
kubectl create -f example/operator/10-crd-operator.gardener.cloud_gardens.yaml
make KUBECONFIG=... start-operator

# now you can create Garden resources, for example
kubectl create -f example/operator/20-garden.yaml
# alternatively, you can run the e2e test
make KUBECONFIG=... test-e2e-local-operator
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

If the `.spec.virtualCluster.controlPlane.highAvailability={}` is set then these components will be deployed in a "highly available" mode.
For ETCD, this means that there will be 3 replicas each.
This works similar like for `Shoot`s (see [this document](../usage/shoot_high_availability.md)) except for the fact that there is no failure tolerance type configurability.
The `gardener-resource-manager`'s [HighAvailabilityConfig webhook](resource-manager.md#high-availability-config) makes sure that all pods with multiple replicas are spread on nodes, and if there are at least two zones in `.spec.runtimeCluster.provider.zones` then they also get spread across availability zones.

> If once set, removing `.spec.virtualCluster.controlPlane.highAvailability` again is not supported.

The `virtual-garden-kube-apiserver` `Deployment` (not yet managed by the operator) is exposed via a `Service` of type `LoadBalancer` with the same name.
In the future, we might switch to exposing it via Istio, similar to how the `kube-apiservers` of shoot clusters are exposed.

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
