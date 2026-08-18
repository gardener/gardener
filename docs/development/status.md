# Status Subresource in Gardener APIs

## Overview

Kubernetes resources are divided into two conceptually separate parts: **spec** (the desired state) and **status** (the observed state). The `status` subresource is a dedicated API endpoint (`/apis/<group>/<version>/<resource>/<name>/status`) that allows controllers to update the observed state independently of the spec.

As a rule, only controllers that observe real-world state should write to `status`.

## Statuses in Gardener APIs

Most Gardener API types carry a `Status` struct that reports what a controller has observed about the resource. While the exact fields vary by resource, several fields appear across many types:

| Field | Description |
|---|---|
| `Conditions` | Health check results (see [Conditions](#conditions)) |
| `Constraints` | Operational signals and informational state (see [Constraints](#constraints)) |
| `LastOperation` | The most recent reconciliation operation (type, state, progress, description) |
| `ObservedGeneration` | The `.metadata.generation` last reconciled by the controller |

## Conditions

Conditions represent the results of **health checks** — discrete observations about whether a specific aspect of the system is working correctly. Each condition has a `Type` (what is being checked), a `Status` (`True`, `False`, `Unknown`, or `Progressing`), a machine-readable `Reason`, and a human-readable `Message`.

```go
type Condition struct {
    Type               ConditionType
    Status             ConditionStatus
    LastTransitionTime metav1.Time
    LastUpdateTime     metav1.Time
    Reason             string
    Message            string
    Codes              []ErrorCode
}
```

`LastTransitionTime` records when `Status` last changed. `LastUpdateTime` records when any field (reason, message, codes) last changed. The `Codes` field carries structured error codes (`ERR_INFRA_QUOTA_EXCEEDED`, `ERR_UNAUTHORIZED`, etc.) that Gardener uses for alerting and user-facing diagnostics.

### Examples

Shoot conditions:

| Type | What it checks |
|---|---|
| `APIServerAvailable` | Whether the shoot's kube-apiserver is reachable |
| `ControlPlaneHealthy` | Health of control plane components (etcd, apiserver, controller-manager, scheduler) |
| `ObservabilityComponentsHealthy` | Health of monitoring and logging components |
| `EveryNodeReady` | Whether all worker nodes are Ready |
| `SystemComponentsHealthy` | Health of system components deployed to the shoot (coredns, vpn, etc.) |

Seed conditions:

| Type | What it checks |
|---|---|
| `BackupBucketsReady` | Whether all backup buckets for the seed are healthy |
| `ExtensionsReady` | Whether all extensions installed in the seed are ready |
| `SeedSystemComponentsHealthy` | Health of system components running in the seed |

### Introducing New Condition Types

Condition types are deliberately stable. Before introducing a new `ConditionType`, check whether an existing type can be reused — many health checks naturally fit under `SystemComponentsHealthy` or `ControlPlaneHealthy`. Fragmenting health checks into many narrow types makes the overall status harder to read.

## Constraints

Constraints use the same `Condition` type and struct as health check conditions but serve a different purpose: they carry **operational signals and informational state** that may gate or influence controller behavior, or that surface relevant cluster information to operators and users.

Unlike conditions, constraints are not limited to binary health checks. They can represent:

- **Prerequisites for operations** — `HibernationPossible` blocks hibernation when `Status: False`; `MaintenancePreconditionsSatisfied` gates maintenance
- **Certificate and credential hygiene** — `CACertificateValiditiesAcceptable` signals when CA certificates are nearing expiry
- **Migration readiness** — `ReadyForMigration` must be `True` before a control-plane migration can proceed
- **Pending user actions** — `ManualInPlaceWorkersUpdated` signals that a worker pool awaits manual intervention
- **Informational signals** — `HasIgnoredManagedResources` or `CRDsWithProblematicConversionWebhooks` inform operators of configuration issues

Also see the [shoot status documentation](../usage/shoot/shoot_status.md) for more information.

### Introducing New Constraint Types

Introducing new constraint types is more common and generally more acceptable than introducing new condition types. Constraints are often specific to an operation or a lifecycle phase, and there is less risk of conflict with existing semantics. If you are implementing a new controller behavior that depends on cluster state, a new constraint type is the right place to surface that signal.

## Conditions and Constraints Status Convention

Gardener's Dashboard and other consumers display conditions and constraints generically. To ensure a consistent visual representation, the following convention applies:

- **Positive results** (everything is fine, the operation is possible) → `Status: True`
- **Negative results** (something is wrong, an operation is blocked) → `Status: False`

Even with this polarity rule, **avoid negating the `Type` name**. A type like `HibernationNotPossible` set to `Status: False` means "hibernation is not not possible" — a double negation that is hard to parse. Instead, use `HibernationPossible` with `Status: False` to express the same state clearly.

## Using the Helper Functions

Never construct or compare `Condition` values by hand. Use the helpers in `pkg/api/core/v1beta1/helper/` to ensure timestamps are set correctly and updates are idempotent.

### Functional helpers (`condition.go`)

`GetOrInitConditionWithClock`, `UpdatedConditionWithClock`, and `MergeConditions` are the most direct way to update a single condition. `UpdatedConditionWithClock` only advances `LastTransitionTime` when `Status` changes and `LastUpdateTime` when `Reason`, `Message`, or `Codes` change — stable inputs produce stable outputs.

See [`pkg/gardenlet/operation/botanist/dualstackmigration.go`](https://github.com/gardener/gardener/blob/ad196442201a759ccb56ead63528b328f57d2638/pkg/gardenlet/operation/botanist/dualstackmigration.go#L77-L79) for a constraint being read, updated, and merged back.

### Builder API (`condition_builder.go`)

`NewConditionBuilder` provides a fluent API useful when the status, reason, and message come from different code paths. `Build()` returns a boolean `updated` that is `true` only when the resulting condition differs from the old one — use it to skip unnecessary status patch calls.

See [`pkg/controllermanager/controller/shoot/migration/reconciler.go`](https://github.com/gardener/gardener/blob/ad196442201a759ccb56ead63528b328f57d2638/pkg/controllermanager/controller/shoot/migration/reconciler.go#L104) for the builder being used to set a constraint.

### Atomically rebuilding both slices

`BuildConditions` removes a set of managed types from the existing slice and appends the freshly computed values, leaving any types owned by other controllers untouched. Use it when a reconciler refreshes all its conditions and constraints in one pass.

See [`pkg/gardenlet/controller/shoot/care/reconciler.go`](https://github.com/gardener/gardener/blob/ad196442201a759ccb56ead63528b328f57d2638/pkg/gardenlet/controller/shoot/care/reconciler.go#L250-L251) for both slices being rebuilt atomically.
