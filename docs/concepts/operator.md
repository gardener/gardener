# Gardener Operator

The `gardener-operator` is meant to be responsible for the garden cluster environment.
Without this component, users must deploy ETCD, the Gardener control plane, etc. manually and with separate mechanisms (not maintained in this repository).
This is quite unfortunate since this requires separate tooling, processes, etc.
A lot of production- and enterprise-grade features were built into Gardener for managing the seed and shoot clusters, so it makes sense to re-use them as much as possible also for the garden cluster.

**⚠️ Consider this component highly experimental and DO NOT use it in production.**

## Deployment

There is a [Helm chart](../../charts/gardener/operator) which can be used to deploy the `gardener-operator`.
Once deployed and ready, you can create a `Garden` resource.

> ℹ️ Similar to seed clusters, garden runtime clusters require a [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
> By default, `gardener-operator` deploys the VPA components.
> However, when there already is a VPA available, then set `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=false` in the `Garden` resource.

## Using Garden Runtime Cluster As Seed Cluster

In production scenarios, you probably wouldn't use the Kubernetes cluster running `gardener-operator` and the Gardener control plane (called "runtime cluster") as seed cluster at the same time.
However, such setup is technically possible and might simplify certain situations (e.g., development, evaluation, ...).

If the runtime cluster is a seed cluster at the same time, [`gardenlet`'s `Seed` controller](../gardenlet.md#seed-controller) will not manage the components which were already deployed (and reconciled) by `gardener-operator`.
As of today, this applies to:

- `gardener-resource-manager`
- `vpa-{admission-controller,recommender,updater}`
- `hvpa-controller` (when `HVPA` feature gate is enabled)

Those components are so-called "seed system components".
As they were already made available by `gardener-operator`, the `gardenlet` just skips them.

> ℹ️ There is no need to configure anything - the `gardenlet` will automatically detect when its seed cluster is the garden runtime cluster at the same time.

⚠️ Note that such setup requires that you upgrade the versions of `gardener-operator` and `gardenlet` in lock-step.
Otherwise, you might experience unexpected behaviour or issues with your seed or shoot clusters.

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

### Control Loops

As of today, the `gardener-operator` only has one controller which is now described in more detail.

#### [`Garden` Controller](../../pkg/operator/controller/garden)

The reconciler first generates a general CA certificate which is valid for ~`30d` and auto-rotated when 80% of its lifetime is reached.
Afterwards, it brings up the so-called "garden system components".
The [`gardener-resource-manager`](../resource-manager.md) is deployed first since its `ManagedResource` controller will be used to bring up the remainders.

Other system components are:
- garden system resources ([`PriorityClass`es](../development/priority-classes.md) for the workload resources)
- Vertical Pod Autoscaler (if enabled via `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=true` in the `Garden`)
- HVPA controller (when `HVPA` feature gate is enabled)

The controller maintains the `Reconciled` condition which indicates the status of an operation.
