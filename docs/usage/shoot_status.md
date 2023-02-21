# Shoot Status

This document provides an overview of the [ShootStatus](../api-reference/core.md#shootstatus).

## Conditions

The Shoot status consists of a set of conditions. A [Condition](../api-reference/core.md#condition) has the following fields:

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
- `ControlPlaneHealthy`
- `EveryNodeReady`
- `SystemComponentsHealthy`

The Shoot conditions are maintained by the [shoot care reconciler](../../pkg/gardenlet/controller/shoot/care/reconciler.go) of gardenlet.
Find more information in [this document](../concepts/gardenlet.md#shoot-controller).

### Sync Period

The condition checks are executed periodically at interval which is configurable in the `GardenletConfiguration` (`.controllers.shootCare.syncPeriod`, defaults to `1m`).

### Condition Thresholds

The `GardenletConfiguration` also allows configuring condition thresholds (`controllers.shootCare.conditionThresholds`). Condition threshold is the amount of time to consider condition as `Processing` on condition status changes.

Let's check the following example to get better understanding. Let's say that the `APIServerAvailable` condition of our Shoot is with status `True`. If the next condition check fails (for example kube-apiserver becomes unreachable), then the condition first goes to `Processing` state. Only if this state remains for condition threshold amount of time, then the condition finally is updated to `False`.

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
Generally, it's [best pratice](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#timeouts) to specify low timeouts in WebhookConfigs.

As an effort to correct this common problem, the webhook remediator has been created. This feature simply checks whether a customer's webhook matches a set of rules, described [here](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/matchers/matcher.go#L66-L180). If at least one of the rule matches, it will change the constraints (`HibernationPossible` and `MaintenancePreconditionsSatisfied`) statuses.

In most cases, you can avoid this by simply excluding the `kube-system` namespace from your webhook:

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

However, some other resources (mostly without namespaces) might still trigger the remediator, namely:
- endpoints
- nodes
- podsecuritypolicies
- clusterroles
- clusterrolebindings
- customresourcedefinitions
- apiservices
- certificatesigningrequests
- priorityclasses

In these cases, please make sure that your `rules` don't overlap with one of those resources (and their subresources).

You can also find help from the [Kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings)

> By setting `.controllers.shootCare.webhookRemediatorEnabled=true` in the gardenlet configuration, the auto-remediation of webhooks not following the best practices can be turned on in the shoot clusters.
> Concretely, missing `namespaceSelector`s or `objectSelector`s will be added and too high `timeoutSeconds` will be lowered.
> In some cases, the `failurePolicy` will be set from `Fail` to `Ignore`.
> Gardenlet will also add an annotation to make it visible to end-users that their webhook configurations were mutated and should be fixed by them in the first place.
> Note that all of this is no perfect solution and just done on a best effort basis.
> Only the owner of the webhook can know whether it indeed is problematic and configured correctly.
> 
> Webhooks labeled with `remediation.webhook.shoot.gardener.cloud/exclude=true` will be excluded from auto-remediation.

**`MaintenancePreconditionsSatisfied`**:

This constraint indicates whether all preconditions for a safe maintenance operation are satisfied (see also [this document](shoot_maintenance.md) for more information about what happens during a shoot maintenance).
As of today, the same checks as in the `HibernationPossible` constraint are being performed (user-deployed webhooks that might interfere with potential rolling updates of shoot worker nodes).
There is no further action being performed on this constraint's status (maintenance is still being performed).
It is meant to make the user aware of potential problems that might occur due to his configurations.

**`CACertificateValiditiesAcceptable`**:

This constraints indicates that there is at least one CA certificate which expires in less than `1y`.
It will not be added to the `.status.constraints` if there is no such CA certificate.
However, if it's visible, then a [credentials rotation operation](shoot_credentials_rotation.md#certificate-authorities) should be considered.

### Last Operation

The Shoot status holds information about the last operation that is performed on the Shoot. The last operation field reflects overall progress and the tasks that are currently being executed. Allowed operation types are `Create`, `Reconcile`, `Delete`, `Migrate` and `Restore`. Allowed operation states are `Processing`, `Succeeded`, `Error`, `Failed`, `Pending` and `Aborted`. An operation in `Error` state is an operation that will be retried for a configurable amount of time (`controllers.shoot.retryDuration` field in `GardenletConfiguration`, defaults to `12h`). If the operation cannot complete successfully for the configured retry duration, it will be marked as `Failed`. An operation in `Failed` state is an operation that won't be retried automatically (to retry such an operation, see [Retry failed operation](https://github.com/gardener/gardener/blob/master/docs/usage/shoot_operations.md#retry-failed-operation)).

### Last Errors

The Shoot status also contains information about the last occurred error(s) (if any) during an operation. A [LastError](../api-reference/core.md#lasterror) consists of identifier of the task returned error, human-readable message of the error and error codes (if any) associated with the error.

### Error Codes

Known error codes are:

- `ERR_INFRA_UNAUTHENTICATED` - indicates that the last error occurred due to the client request not being completed because it lacks valid authentication credentials for the requested resource. It is classified as a non-retryable error code.
- `ERR_INFRA_UNAUTHORIZED` - indicates that the last error occurred due to the server understanding the request but refusing to authorize it. It is classified as a non-retryable error code.
- `ERR_INFRA_QUOTA_EXCEEDED` - indicates that the last error occurred due to infrastructure quota limits. It is classified as a non-retryable error code.
- `ERR_INFRA_RATE_LIMITS_EXCEEDED` - indicates that the last error occurred due to exceeded infrastructure request rate limits.
- `ERR_INFRA_DEPENDENCIES` - indicates that the last error occurred due to dependent objects on the infrastructure level. It is classified as a non-retryable error code.
- `ERR_RETRYABLE_INFRA_DEPENDENCIES` - indicates that the last error occurred due to dependent objects on the infrastructure level, but the operation should be retried.
- `ERR_INFRA_RESOURCES_DEPLETED` - indicates that the last error occurred due to depleted resource in the infrastructure.
- `ERR_CLEANUP_CLUSTER_RESOURCES` - indicates that the last error occurred due to resources in the cluster that are stuck in deletion.
- `ERR_CONFIGURATION_PROBLEM` - indicates that the last error occurred due to a configuration problem. It is classified as a non-retryable error code.
- `ERR_RETRYABLE_CONFIGURATION_PROBLEM` - indicates that the last error occurred due to a retryable configuration problem. "Retryable" means that the occurred error is likely to be resolved in a ungraceful manner after given period of time.
- `ERR_PROBLEMATIC_WEBHOOK` - indicates that the last error occurred due to a webhook not following the Kubernetes best practices (https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings).

### Status Label

Shoots will be automatically labeled with the `shoot.gardener.cloud/status` label.
Its value might either be `healthy`, `progressing`, `unhealthy` or `unknown` depending on the `.status.conditions`, `.status.lastOperation` and `status.lastErrors` of the `Shoot`.
This can be used as an easy filter method to find shoots based on their "health" status.
