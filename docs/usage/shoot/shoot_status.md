---
description: Shoot conditions, constraints, and error codes
---

# Shoot Status

This document provides an overview of the [ShootStatus](../../api-reference/core.md#shootstatus).

## Conditions

The Shoot status consists of a set of conditions. A [Condition](../../api-reference/core.md#condition) has the following fields:

| Field name           | Description                                                                                                        |
| -------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `type`               | Name of the condition.                                                                                             |
| `status`             | Indicates whether the condition is applicable, with possible values `True`, `False`, `Unknown` or `Progressing`.  |
| `lastTransitionTime` | Timestamp for when the condition last transitioned from one status to another.                                     |
| `lastUpdateTime`     | Timestamp for when the condition was updated. Usually changes when `reason` or `message` in condition is updated.  |
| `reason`             | Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.                   |
| `message`            | Human-readable message indicating details about the last status transition.                                        |
| `codes`              | Well-defined error codes in case the condition reports a problem.                                                  |

Currently, the available Shoot condition types are:

- `APIServerAvailable`
- `ControlPlaneHealthy`
- `EveryNodeReady`
- `ObservabilityComponentsHealthy`
- `SystemComponentsHealthy`

The Shoot conditions are maintained by the [shoot care reconciler](../../../pkg/gardenlet/controller/shoot/care/reconciler.go) of the gardenlet.
Find more information in the [gardelent documentation](../../concepts/gardenlet.md#shoot-controller).

### Sync Period

The condition checks are executed periodically at an interval which is configurable in the `GardenletConfiguration` (`.controllers.shootCare.syncPeriod`, defaults to `1m`).

### Condition Thresholds

The `GardenletConfiguration` also allows configuring condition thresholds (`controllers.shootCare.conditionThresholds`). A condition threshold is the amount of time to consider a condition as `Processing` on condition status changes.

Let's check the following example to get a better understanding. Let's say that the `APIServerAvailable` condition of our Shoot is with status `True`. If the next condition check fails (for example kube-apiserver becomes unreachable), then the condition first goes to `Processing` state. Only if this state remains for condition threshold amount of time, then the condition is finally updated to `False`.

### Constraints

Constraints represent conditions of a Shoot’s current state that constraint some operations on it.
The current constraints are:

**`HibernationPossible`**:

This constraint indicates whether a Shoot is allowed to be hibernated.
The rationale behind this constraint is that a Shoot can have `ValidatingWebhookConfiguration`s or `MutatingWebhookConfiguration`s acting on resources that are critical for waking up a cluster.
For example, if a webhook has rules for `CREATE/UPDATE` Pods or Nodes and `failurePolicy=Fail`, the webhook will block joining `Nodes` and creating critical system component Pods and thus block the entire wakeup operation, because the server backing the webhook is not running.

Even if the `failurePolicy` is set to `Ignore`, high timeouts (`>15s`) can lead to blocking requests of control plane components.
That's because most control-plane API calls are made with a client-side timeout of `30s`, so if a webhook has `timeoutSeconds=30`
the overall request might still fail as there is overhead in communication with the API server and potential other webhooks.

Generally, it's [best practice](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#timeouts) to specify low timeouts in WebhookConfigs.

As an effort to correct this common problem, the webhook remediator has been created. This is enabled by setting `.controllers.shootCare.webhookRemediatorEnabled=true` in the `gardenlet`'s configuration. This feature simply checks whether webhook configurations in shoot clusters match a set of rules described [here](../../../pkg/gardenlet/operation/botanist/matchers/matcher.go). If at least one of the rules matches, it will change set `status=False` for the `.status.constraints` of type `HibernationPossible` and `MaintenancePreconditionsSatisfied` in the `Shoot` resource. In addition, the `failurePolicy` in the affected webhook configurations will be set from `Fail` to `Ignore`. Gardenlet will also add an annotation to make it visible to end-users that their webhook configurations were mutated and should be fixed/adapted according to the rules and best practices.

In most cases, you can avoid this by simply excluding the `kube-system` namespace from your webhook via the `namespaceSelector`:
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
webhooks:
  - name: my-webhook.example.com
    namespaceSelector:
      matchExpressions:
      - key: gardener.cloud/purpose
        operator: NotIn
        values:
          - kube-system
    rules:
      - operations: ["*"]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
        scope: "Namespaced"
```
However, some other resources (some of them cluster-scoped) might still trigger the remediator, namely:
- endpoints
- nodes
- clusterroles
- clusterrolebindings
- customresourcedefinitions
- apiservices
- certificatesigningrequests
- priorityclasses

If one of the above resources triggers the remediator, the preferred solution is to remove that particular resource from your webhook's `rules`. You can also use the `objectSelector` to reduce the scope of webhook's `rules`. However, in special cases where a webhook is absolutely needed for the workload, it is possible to add the `remediation.webhook.shoot.gardener.cloud/exclude=true` label to your webhook so that the remediator ignores it. This label **should not be used to silence an alert**, but rather to confirm that a webhook won't cause problems. Note that all of this is no perfect solution and just done on a best effort basis, and only the owner of the webhook can know whether it indeed is problematic and configured correctly.

In a special case, if a webhook has a rule for `CREATE/UPDATE` lease resources in `kube-system` namespace, its `timeoutSeconds` is updated to 3 seconds. This is required to ensure the proper functioning of the leader election of essential control plane controllers.

You can also find more help from the [Kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings)

**`MaintenancePreconditionsSatisfied`**:

This constraint indicates whether all preconditions for a safe maintenance operation are satisfied (see [Shoot Maintenance](shoot_maintenance.md) for more information about what happens during a shoot maintenance).
As of today, the same checks as in the `HibernationPossible` constraint are being performed (user-deployed webhooks that might interfere with potential rolling updates of shoot worker nodes).
There is no further action being performed on this constraint's status (maintenance is still being performed).
It is meant to make the user aware of potential problems that might occur due to his configurations.

**`CACertificateValiditiesAcceptable`**:

This constraint indicates that there is at least one CA certificate which expires in less than `1y`.
It will not be added to the `.status.constraints` if there is no such CA certificate.
However, if it's visible, then a [credentials rotation operation](../shoot-operations/shoot_credentials_rotation.md#certificate-authorities) should be considered.

**`CRDsWithProblematicConversionWebhooks`**:

This constraint indicates that there is at least one `CustomResourceDefinition` in the cluster which has multiple stored versions and a conversion webhook configured. This could break the reconciliation flow of a `Shoot` cluster in some cases. See https://github.com/gardener/gardener/issues/7471 for more details.
It will not be added to the `.status.constraints` if there is no such CRD.
However, if it's visible, then you should consider upgrading the existing objects to the current stored version. See [Upgrade existing objects to a new stored version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version) for detailed steps.

**`ManualInPlaceWorkersUpdated`**:

This constraint indicates that at least one worker pool with the update strategy `ManualInPlaceUpdate` is pending an update. Despite this, the `Shoot` reconciliation will still succeed.
The constraint is not added to `.status.constraints` if all such worker pools are already up-to-date.
Once the user manually labels all the relevant nodes with `node.machine.sapcloud.io/selected-for-update` and the update process completes, the constraint will be automatically removed.

### Last Operation

The Shoot status holds information about the last operation that is performed on the Shoot. The last operation field reflects overall progress and the tasks that are currently being executed. Allowed operation types are `Create`, `Reconcile`, `Delete`, `Migrate`, and `Restore`. Allowed operation states are `Processing`, `Succeeded`, `Error`, `Failed`, `Pending`, and `Aborted`. An operation in `Error` state is an operation that will be retried for a configurable amount of time (`controllers.shoot.retryDuration` field in `GardenletConfiguration`, defaults to `12h`). If the operation cannot complete successfully for the configured retry duration, it will be marked as `Failed`. An operation in `Failed` state is an operation that won't be retried automatically (to retry such an operation, see [Retry failed operation](../shoot-operations/shoot_operations.md#retry-failed-operation)).

### Last Errors

The Shoot status also contains information about the last occurred error(s) (if any) during an operation. A [LastError](../../api-reference/core.md#lasterror) consists of identifier of the task returned error, human-readable message of the error and error codes (if any) associated with the error.

### Error Codes

Known error codes and their classification are:

| Error code                            | User error | Description                                                                                         |
| ------------------------------------- | :--------: | --------------------------------------------------------------------------------------------------- |
| `ERR_INFRA_UNAUTHENTICATED`           | true       | Indicates that the last error occurred due to the client request not being completed because it lacks valid authentication credentials for the requested resource. It is classified as a non-retryable error code. |
| `ERR_INFRA_UNAUTHORIZED`              | true       | Indicates that the last error occurred due to the server understanding the request but refusing to authorize it. It is classified as a non-retryable error code. |
| `ERR_INFRA_QUOTA_EXCEEDED`            | true       | Indicates that the last error occurred due to infrastructure quota limits. It is classified as a non-retryable error code. |
| `ERR_INFRA_RATE_LIMITS_EXCEEDED`      | false      | Indicates that the last error occurred due to exceeded infrastructure request rate limits. |
| `ERR_INFRA_DEPENDENCIES`              | true       | Indicates that the last error occurred due to dependent objects on the infrastructure level. It is classified as a non-retryable error code. |
| `ERR_RETRYABLE_INFRA_DEPENDENCIES`    | false      | Indicates that the last error occurred due to dependent objects on the infrastructure level, but the operation should be retried. |
| `ERR_INFRA_RESOURCES_DEPLETED`        | true       | Indicates that the last error occurred due to depleted resource in the infrastructure. |
| `ERR_CLEANUP_CLUSTER_RESOURCES`       | true       | Indicates that the last error occurred due to resources in the cluster that are stuck in deletion. |
| `ERR_CONFIGURATION_PROBLEM`           | true       | Indicates that the last error occurred due to a configuration problem. It is classified as a non-retryable error code. |
| `ERR_RETRYABLE_CONFIGURATION_PROBLEM` | true       | Indicates that the last error occurred due to a retryable configuration problem. "Retryable" means that the occurred error is likely to be resolved in a ungraceful manner after given period of time. |
| `ERR_PROBLEMATIC_WEBHOOK`             | true       | Indicates that the last error occurred due to a webhook not following the [Kubernetes best practices](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings). |

**Please note:** Errors classified as `User error: true` do not require a Gardener operator to resolve but can be remediated by the user (e.g. by refreshing expired infrastructure credentials).
Even though `ERR_INFRA_RATE_LIMITS_EXCEEDED` and `ERR_RETRYABLE_INFRA_DEPENDENCIES` is mentioned as User error: false` operator can't provide any resolution because it is related to cloud provider issue.

### Status Label

Shoots will be automatically labeled with the `shoot.gardener.cloud/status` label.
Its value might either be `healthy`, `progressing`, `unhealthy` or `unknown` depending on the `.status.conditions`, `.status.lastOperation`, and `status.lastErrors` of the `Shoot`.
This can be used as an easy filter method to find shoots based on their "health" status.
