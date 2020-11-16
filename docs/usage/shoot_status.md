# Shoot Status

This document provides an overview of the [ShootStatus](https://gardener.cloud/documentation/references/core/#core.gardener.cloud/v1beta1.ShootStatus).

## Conditions

The Shoot status consists of a set of conditions. A [Condition](https://gardener.cloud/documentation/references/core/#core.gardener.cloud/v1beta1.Condition) has the following fields:

| Field name           | Description                                                                                                        |
| -------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `type`               | Name of the condition.                                                                                             |
| `status`             | Indicates whether the condition is applicable, with possible values `True`, `False`, `Unknown`, or `Progressing`.  |
| `lastTransitionTime` | Timestamp for when the condition last transitioned from one status to another.                                     |
| `lastUpdateTime`     | Timestamp for when the condition was updated. Usually changes when `reason` or `message` in condition is updated.  |
| `reason`             | Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.                   |
| `message`            | Human-readable message indicating details about the last status transition.                                        |
| `codes`              | Well-defined error codes in case the condition reports a problem.                                                  |

Currently the available Shoot condition types are:

- `APIServerAvailable`

  This condition type indicates whether the Shoot's kube-apiserver is available or not. In particular, the `/healthz` endpoint of the kube-apiserver is called, and the expected response code is `HTTP 200`.

- `ControlPlaneHealthy`

  This condition type indicates whether all the control plane components deployed to the Shoot's namespace in the Seed do exist and are running fine.

- `EveryNodeReady`

  This condition type indicates whether at least the requested minimum number of Nodes is present per each worker pool and whether all Nodes are healthy.

- `SystemComponentsHealthy`

  This condition type indicates whether all system components deployed to the `kube-system` namespace in the shoot do exist and are running fine. It also reflects whether the tunnel connection between the control plane and the Shoot networks can be established.

The Shoot conditions are maintained by the [shoot care control](https://github.com/gardener/gardener/blob/master/pkg/gardenlet/controller/shoot/shoot_care_control.go) of gardenlet.

### Sync Period

The condition checks are executed periodically at interval which is configurable in the `GardenletConfiguration` (`.controllers.shootCare.syncPeriod`, defaults to `1m`).

### Condition Thresholds

The `GardenletConfiguration` also allows configuring condition thresholds (`controllers.shootCare.conditionThresholds`). Condition threshold is the amount of time to consider condition as `Processing` on condition status changes.

Let's check the following example to get better understanding. Let's say that the `APIServerAvailable` condition of our Shoot is with status `True`. If the next condition check fails (for example kube-apiserver becomes unreachable), then the condition first goes to `Processing` state. Only if this state remains for condition threshold amount of time, then the condition finally is updated to `False`.

### Constraints

Constraints represent conditions of a Shoot’s current state that constraint some operations on it.

Currently there are two constraints:

- `HibernationPossible`

  This constraint indicates whether a Shoot is allowed to be hibernated. The rationale behind this constraint is that a Shoot can have `ValidatingWebhookConfiguration`s or `MutatingWebhookConfiguration`s with rules for CREATE Pods or Nodes and `failurePolicy=Fail`. Such webhooks are preventing wake up for a Shoot cluster as new Nodes cannot be created or new system component Pods cannot be created because the webhook is not running. To prevent such deadlock situation, the gardener-apiserver does not allow Shoot to be hibernated when the `HibernationPossible` has status `False`.

- `MaintenancePreconditionsSatisfied`

  This constraint indicates whether all preconditions for a safe maintenance operation are satisfied (see also [this document](shoot_maintenance.md) for more information about what happens during a shoot maintenance). As of today, the same checks as in the `HibernationPossible` constraint are being performed (user-deployed webhooks that might interfere with potential rolling updates of shoot worker nodes). There is no further action being performed on this constraint's status (maintenance is still being performed). It is meant to make the user aware of potential problems that might occur due to his configurations. 

### Last Operation

The Shoot status holds information about the last operation that is performed on the Shoot. The last operation field reflects overall progress and the tasks that are currently being executed. Allowed operation types are `Create`, `Reconcile`, `Delete`, `Migrate` and `Restore`. Allowed operation states are `Processing`, `Succeeded`, `Error`, `Failed`, `Pending` and `Aborted`. An operation in `Error` state is an operation that will be retried for a configurable amount of time (`controllers.shoot.retryDuration` field in `GardenletConfiguration`, defaults to `12h`). If the operation cannot complete successfully for the configured retry duration, it will be marked as `Failed`. An operation in `Failed` state is an operation that won't be retried automatically (to retry such an operation, see [Retry failed operation](https://github.com/gardener/gardener/blob/master/docs/usage/shoot_operations.md#retry-failed-operation)).

### Last Errors

The Shoot status also contains information about the last occurred error(s) (if any) during an operation. A [LastError](https://gardener.cloud/documentation/references/core/#core.gardener.cloud/v1beta1.LastError) consists of identifier of the task returned error, human-readable message of the error and error codes (if any) associated with the error.

### Error Codes

Known error codes are:

- `ERR_INFRA_UNAUTHORIZED` - indicates that the last error occurred due to invalid infrastructure credentials
- `ERR_INFRA_INSUFFICIENT_PRIVILEGES` - indicates that the last error occurred due to insufficient infrastructure privileges
- `ERR_INFRA_QUOTA_EXCEEDED` - indicates that the last error occurred due to infrastructure quota limits
- `ERR_INFRA_DEPENDENCIES` - indicates that the last error occurred due to dependent objects on the infrastructure level
- `ERR_INFRA_RESOURCES_DEPLETED` - indicates that the last error occurred due to depleted resource in the infrastructure
- `ERR_CLEANUP_CLUSTER_RESOURCES` - indicates that the last error occurred due to resources in the cluster that are stuck in deletion
- `ERR_CONFIGURATION_PROBLEM` - indicates that the last error occurred due to a configuration problem
