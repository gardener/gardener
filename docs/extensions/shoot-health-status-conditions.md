# Contributing to Shoot Health Status Conditions

Gardener checks regularly (every minute by default) the health status of all shoot clusters.
It categorizes its checks into five different types:

* `APIServerAvailable`: This type indicates whether the shoot's kube-apiserver is available or not.
* `ControlPlaneHealthy`: This type indicates whether the core components of the Shoot controlplane (ETCD, KAPI, KCM..) are healthy.
* `EveryNodeReady`: This type indicates whether all `Node`s and all `Machine` objects report healthiness.
* `ObservabilityComponentsHealthy`: This type indicates whether the  observability components of the Shoot control plane (Prometheus, Vali, Plutono..) are healthy.
* `SystemComponentsHealthy`: This type indicates whether all system components deployed to the `kube-system` namespace in the shoot do exist and are running fine.

In case of workerless `Shoot`, `EveryNodeReady` condition is not present in the `Shoot`'s conditions since there are no nodes in the cluster.

Every `Shoot` resource has a `status.conditions[]` list that contains the mentioned types, together with a `status` (`True`/`False`) and a descriptive message/explanation of the `status`.

Most extension controllers are deploying components and resources as part of their reconciliation flows into the seed or shoot cluster.
A prominent example for this is the `ControlPlane` controller that usually deploys a cloud-controller-manager or CSI controllers as part of the shoot control plane.
Now that the extensions deploy resources into the cluster, especially resources that are essential for the functionality of the cluster, they might want to contribute to Gardener's checks mentioned above.

## What can extensions do to contribute to Gardener's health checks?

Every extension resource in Gardener's `extensions.gardener.cloud/v1alpha1` API group also has a `status.conditions[]` list (like the `Shoot`).
Extension controllers can write conditions to the resource they are acting on and use a type that also exists in the shoot's conditions.
One exception is that `APIServerAvailable` can't be used, as Gardener clearly can identify the status of this condition and it doesn't make sense for extensions to try to contribute/modify it.

As an example for the `ControlPlane` controller, let's take a look at the following resource:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: ControlPlane
metadata:
  name: control-plane
  namespace: shoot--foo--bar
spec:
  ...
status:
  conditions:
  - type: ControlPlaneHealthy
    status: "False"
    reason: DeploymentUnhealthy
    message: 'Deployment cloud-controller-manager is unhealthy: condition "Available" has
      invalid status False (expected True) due to MinimumReplicasUnavailable: Deployment
      does not have minimum availability.'
    lastUpdateTime: "2014-05-25T12:44:27Z"
  - type: ConfigComputedSuccessfully
    status: "True"
    reason: ConfigCreated
    message: The cloud-provider-config has been successfully computed.
    lastUpdateTime: "2014-05-25T12:43:27Z"
```

The extension controller has declared in its extension resource that one of the deployments it is responsible for is unhealthy.
Also, it has written a second condition using a type that is unknown by Gardener.

Gardener will pick the list of conditions and recognize that there is one with a type `ControlPlaneHealthy`.
It will merge it with its own `ControlPlaneHealthy` condition and report it back to the `Shoot`'s status:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  labels:
    shoot.gardener.cloud/status: unhealthy
  name: some-shoot
  namespace: garden-core
spec:
status:
  conditions:
  - type: APIServerAvailable
    status: "True"
    reason: HealthzRequestSucceeded
    message: API server /healthz endpoint responded with success status code. [response_time:31ms]
    lastUpdateTime: "2014-05-23T08:26:52Z"
    lastTransitionTime: "2014-05-25T12:45:13Z"
  - type: ControlPlaneHealthy
    status: "False"
    reason: ControlPlaneUnhealthyReport
    message: 'Deployment cloud-controller-manager is unhealthy: condition "Available" has
      invalid status False (expected True) due to MinimumReplicasUnavailable: Deployment
      does not have minimum availability.'
    lastUpdateTime: "2014-05-25T12:45:13Z"
    lastTransitionTime: "2014-05-25T12:45:13Z"
  ...
```

Hence, the only duty extensions have is to maintain the health status of their components in the extension resource they are managing.
This can be accomplished using the [health check library for extensions](./healthcheck-library.md).

## Error Codes

The Gardener API includes some well-defined error codes, e.g., `ERR_INFRA_UNAUTHORIZED`, `ERR_INFRA_DEPENDENCIES`, etc.
Extension may set these error codes in the `.status.conditions[].codes[]` list in case it makes sense.
Gardener will pick them up and will similarly merge them into the `.status.conditions[].codes[]` list in the `Shoot`:

```yaml
status:
  conditions:
  - type: ControlPlaneHealthy
    status: "False"
    reason: DeploymentUnhealthy
    message: 'Deployment cloud-controller-manager is unhealthy: condition "Available" has
      invalid status False (expected True) due to MinimumReplicasUnavailable: Deployment
      does not have minimum availability.'
    lastUpdateTime: "2014-05-25T12:44:27Z"
    codes:
    - ERR_INFRA_UNAUTHORIZED 
``` 
