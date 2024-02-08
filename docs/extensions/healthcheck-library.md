# Health Check Library

## Goal

Typically, an extension reconciles a specific resource (Custom Resource Definitions (CRDs)) and creates / modifies resources in the cluster (via helm, managed resources, kubectl, ...).
We call these API Objects 'dependent objects' - as they are bound to the lifecycle of the extension.

The goal of this library is to enable extensions to setup health checks for their 'dependent objects' with minimal effort.

## Usage

The library provides a generic controller with the ability to register any resource that satisfies the [extension object interface](../../pkg/apis/extensions/v1alpha1/types.go).
An example is [the `Worker` CRD](../../pkg/apis/extensions/v1alpha1/types_worker.go).

Health check functions for commonly used dependent objects can be reused and registered with the controller, such as:

- Deployment
- DaemonSet
- StatefulSet
- ManagedResource (Gardener specific)

See the below example [taken from the provider-aws](https://github.com/gardener/gardener-extension-provider-aws/blob/master/pkg/controller/healthcheck/add.go).

```go
health.DefaultRegisterExtensionForHealthCheck(
               aws.Type,
               extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource),
               func() runtime.Object { return &extensionsv1alpha1.Worker{} },
               mgr, // controller runtime manager
               opts, // options for the health check controller
               nil, // custom predicates
               map[extensionshealthcheckcontroller.HealthCheck]string{
                       general.CheckManagedResource(genericactuator.McmShootResourceName): string(gardencorev1beta1.ShootSystemComponentsHealthy),
                       general.CheckSeedDeployment(aws.MachineControllerManagerName):      string(gardencorev1beta1.ShootEveryNodeReady),
                       worker.SufficientNodesAvailable():                                  string(gardencorev1beta1.ShootEveryNodeReady),
               })
```

This creates a health check controller that reconciles the `extensions.gardener.cloud/v1alpha1.Worker` resource with the spec.type 'aws'.
Three health check functions are registered that are executed during reconciliation.
Each health check is mapped to a single `HealthConditionType` that results in conditions with the same `condition.type` (see below).
To contribute to the Shoot's health, the following conditions can be used: `SystemComponentsHealthy`, `EveryNodeReady`, `ControlPlaneHealthy`, `ObservabilityComponentsHealthy`. In case of workerless `Shoot` the `EveryNodeReady` condition is not present, so it can't be used.

The Gardener/Gardenlet checks each extension for conditions matching these types.
However, extensions are free to choose any `HealthConditionType`.
For more information, see [Contributing to Shoot Health Status Conditions](./shoot-health-status-conditions.md).

A health check has to [satisfy the below interface](../../extensions/pkg/controller/healthcheck/actuator.go).
You can find implementation examples in the [healtcheck folder](../../extensions/pkg/controller/healthcheck/general).

```go
type HealthCheck interface {
    // Check is the function that executes the actual health check
    Check(context.Context, types.NamespacedName) (*SingleCheckResult, error)
    // InjectSeedClient injects the seed client
    InjectSeedClient(client.Client)
    // InjectShootClient injects the shoot client
    InjectShootClient(client.Client)
    // SetLoggerSuffix injects the logger
    SetLoggerSuffix(string, string)
    // DeepCopy clones the healthCheck
    DeepCopy() HealthCheck
}
```

The health check controller regularly (default: `30s`) reconciles the extension resource and executes the registered health checks for the dependent objects.
As a result, the controller writes condition(s) to the status of the extension containing the health check result.
In our example, two checks are mapped to `ShootEveryNodeReady` and one to `ShootSystemComponentsHealthy`, leading to conditions with two distinct `HealthConditionTypes` (condition.type):

```yaml
status:
  conditions:
    - lastTransitionTime: "20XX-10-28T08:17:21Z"
      lastUpdateTime: "20XX-11-28T08:17:21Z"
      message: (1/1) Health checks successful
      reason: HealthCheckSuccessful
      status: "True"
      type: SystemComponentsHealthy
    - lastTransitionTime: "20XX-10-28T08:17:21Z"
      lastUpdateTime: "20XX-11-28T08:17:21Z"
      message: (2/2) Health checks successful
      reason: HealthCheckSuccessful
      status: "True"
      type: EveryNodeReady
```

Please note that there are four statuses: `True`, `False`, `Unknown`, and `Progressing`.

- `True` should be used for successful health checks.
- `False` should be used for unsuccessful/failing health checks.
- `Unknown` should be used when there was an error trying to determine the health status.
- `Progressing` should be used to indicate that the health status did not succeed but for expected reasons (e.g., a cluster scale up/down could make the standard health check fail because something is wrong with the `Machines`, however, it's actually an expected situation and known to be completed within a few minutes.)

Health checks that report `Progressing` should also provide a timeout, after which this "progressing situation" is expected to be completed.
The health check library will automatically transition the status to `False` if the timeout was exceeded.

## Additional Considerations

It is up to the extension to decide how to conduct health checks, though it is recommended to make use of the build-in health check functionality of `managed-resources` for trivial checks.
By [deploying the depending resources via managed resources](../../extensions/pkg/controller/worker/genericactuator/machine_controller_manager.go), the [gardener resource manager](https://github.com/gardener/gardener-resource-manager) conducts basic checks for different API objects out-of-the-box (e.g `Deployments`, `DaemonSets`, ...) - and writes health conditions.
By default, Gardener performs health check for all the `ManagedResource`s with `.spec.class=nil` created in the shoot namespaces.

More sophisticated health checks should be implemented by the extension controller itself (implementing the `HealthCheck` interface).
