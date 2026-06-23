---
title: Shoot Machine Preservation
description: Retaining unhealthy worker nodes for debugging and analysis using manual or automatic machine preservation
---

# Shoot Machine Preservation

This document describes how to configure machine preservation for shoot cluster worker nodes, enabling operators and end-users to retain failed (or running) machines and their backing VMs for debugging and analysis.


## What Is Machine Preservation?

Normally, when a machine enters the `Failed` phase, Machine Controller Manager (MCM) terminates it and creates a replacement. Machine preservation suspends this termination, giving you time to inspect the failing node, collect logs, or attempt recovery.

A preserved machine/node has the following properties:

- **`Failed` phase:** The machine stays in `Failed` until the preservation timeout expires without being terminated. MCM drains all pods from the backing node except DaemonSet pods. The node is cordoned to prevent new pod scheduling. The Cluster Autoscaler (CA) `scale-down-disabled` annotation is added to prevent accidental removal.
- **`Running` phase:** If a machine is preserved while running (e.g., annotated with `now`), the CA `scale-down-disabled` annotation is added to prevent underutilisation-driven scale-down.
- **Scale-down preference:** When a MachineDeployment must scale down, preserved machines are de-prioritised and are the last to be removed.
- **Replica counting:** Preserved machines count toward the desired replica count of their MachineDeployment.

> **Note:** Preservation does not prevent explicit deletion. If you run `kubectl delete machine` or `kubectl delete node`, the machine and its backing VM will be deleted.

## Effect on Shoot Conditions

Preserved failed machines affect three Shoot conditions: `NoPreservedFailedMachines`, `EveryNodeReady`, and `SystemComponentsHealthy`.

### `NoPreservedFailedMachines`

When auto-preservation is enabled (i.e., `autoPreserveFailedMachineMax > 0`) for any worker pool in the Shoot, Gardener tracks preserved failed machines and exposes a `NoPreservedFailedMachines` condition in the Shoot status.

| Condition status | Meaning |
|---|---|
| `True` | No failed machines are currently preserved. |
| `False` | One or more failed machines are currently being preserved. |

### `EveryNodeReady`

The `EveryNodeReady` condition reflects whether all registered nodes are ready and healthy. Preservation changes how failures in this condition are reported:

- If **all** unhealthy nodes are preserved, the `EveryNodeReady` failure is buffered rather than returned immediately. The condition is set to `False` only after all other node checks (node-agent leases, systemd units, scaling, expired leases) pass. This ensures that a real, non-preservation-related problem in those checks is surfaced first.
- If **any** unhealthy node is not preserved, the failure is returned immediately, as usual.
- Preserved nodes are excluded from node-agent lease, systemd-unit, and expired-lease checks (since they are expected to be unhealthy), but are still counted in node-scaling checks so that cordoned preserved nodes are correctly accounted for in scale-down calculations.
- When an `EveryNodeReady` failure is attributable to a preserved node, the condition message includes the suffix `(node and backing machine preserved by MCM)` to indicate the cause.

### `SystemComponentsHealthy`

When `NoPreservedFailedMachines` is `False`, Gardener suppresses `SystemComponentsHealthy` failures that are attributable solely to DaemonSet pods running on the preserved (unhealthy) nodes. This prevents a single preserved failed node from blocking the overall health of the cluster. Specifically:

- For each unhealthy ManagedResource, suppression is evaluated per resource type:
  - **DaemonSet:** failures caused entirely by pods on preserved nodes are suppressed.
  - **Deployment / StatefulSet:** failures are never suppressed — these are not node-specific.
  - **Any other resource kind:** failures are never suppressed.
- If a ManagedResource contains a mix of resource types, suppression applies only when all resources within that ManagedResource pass their respective checks.

## Configuration

Preservation is configured per worker pool in the Shoot spec. Two fields control it:

- **`spec.provider.workers[].autoPreserveFailedMachineMax`** — the maximum number of `Failed` machines MCM may auto-preserve concurrently in this worker pool.
- **`spec.provider.workers[].machineControllerManager.machinePreserveTimeout`** — how long a machine stays preserved before MCM automatically stops preservation. Defaults to `96h` (4 days) if not set. Must be a positive duration.

### Constraints on `autoPreserveFailedMachineMax`

| Condition | Maximum allowed value |
|---|---|
| Worker pool allows system components (`systemComponents.allow: true`) | `Maximum - 1` — at least one machine must remain available to run system components |
| Worker pool does not allow system components | `Maximum` |

`autoPreserveFailedMachineMax` defaults to `0`, meaning auto-preservation is disabled. The configured value is distributed across zones (MachineDeployments) in the worker pool using the same proportional distribution as `minimum` and `maximum`. If the limit is reached, additional failed machines are not preserved and are terminated normally.

### Example

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: garden-my-project
spec:
  provider:
    workers:
    - name: worker-pool-1
      minimum: 1
      maximum: 5
      autoPreserveFailedMachineMax: 2
      machineControllerManager:
        machinePreserveTimeout: 72h
```

> **Note:** Changes to `machinePreserveTimeout` apply only to preservations that start after the change. Existing preserved machines are not affected. If you need to extend or shorten the preservation window of a currently preserved machine, edit the `preserveExpiryTime` field in the machine's status directly.

## Preservation Modes

Preservation can be triggered in two ways:

### Automatic Preservation by MCM

When `autoPreserveFailedMachineMax > 0`, MCM automatically annotates failed machines with `node.machine.sapcloud.io/preserve=auto-preserved` up to the configured limit. These machines count toward `autoPreserveFailedMachineMax`.

If `autoPreserveFailedMachineMax` is decreased after some machines are already auto-preserved, MCM de-preserves auto-preserved machines until the count is within the new limit. When choosing which machine to de-preserve among two machines of the same preservation type, the one with the earlier `preserveExpiryTime` is de-preserved first.

### Manual Preservation

Operators and end-users can preserve individual machines by annotating the machine or its backing node object. Annotating the node is preferred when the node exists and is accessible, to avoid ambiguity.

**Annotation key:** `node.machine.sapcloud.io/preserve`

| Annotation value | Effect                                                                                   |
|------------------|------------------------------------------------------------------------------------------|
| `when-failed`    | Preserve the machine only when (or if) it enters `Failed` phase                          |
| `now`            | Preserve the machine immediately, regardless of its current phase                        |
| `false`          | Explicitly opt out of auto-preservation for this machine; also stops active preservation |
| `auto-preserved` | Set by MCM only — indicates this machine is being preserved due to auto-preservation     |

#### Annotation priority

If both the machine and the node have the preservation annotation, the **node's annotation value takes precedence** and the machine's annotation is removed. To avoid unexpected behaviour, annotate the node object when it is accessible.

#### When preservation stops

When `preserveExpiryTime` is reached, MCM stops preservation and removes the annotation from the machine or node to prevent the same machine from being re-preserved unintentionally.

#### Behaviour on recovery from `Failed` to `Running`

| Annotation value | Behaviour on recovery                                                                               |
|------------------|-----------------------------------------------------------------------------------------------------|
| `now`            | Preservation continues until `preserveExpiryTime`                                                   |
| `when-failed`    | Preservation stops; annotation is retained so the machine will be preserved again if it fails again |
| `auto-preserved` | Preservation stops; MCM sets this value only for the `Failed` phase                                 |

In all cases, when the machine transitions to `Running`, the backing node is uncordoned to allow pod scheduling to resume.

> **Note:** When preservation stops and the machine is `Running`, CA's `scale-down-unneeded-time` applies. If the node remains under the utilisation threshold after that period, CA may scale it down.

## Stopping Preservation Early

### Manual preservation

Delete the preservation annotation from whichever object (node or machine) carries it.

### Auto-preservation

Annotate the machine or node with `node.machine.sapcloud.io/preserve=false`. This instructs MCM to stop auto-preservation and marks the machine as ineligible for future auto-preservation. Deleting the annotation is not sufficient — MCM would re-preserve the machine if it is still in Failed phase, provided the limit permits.

## Preventing Auto-Preservation

To prevent a specific machine from ever being auto-preserved, annotate it (or its backing node) with:

```
node.machine.sapcloud.io/preserve=false
```


> **Warning:** When `autoPreserveFailedMachineMax > 0` is set on any worker pool, Gardener disables the Cluster Autoscaler's cluster health check (`--max-total-unready-percentage=100`). This allows unscheduled workload from preserved failed nodes to trigger scale-up, but it also means CA will continue scaling even when a large fraction of nodes are unready.

## Limitations

- **Rolling updates:** Preservation is ignored during rolling updates. Failed machines are replaced as usual regardless of preservation settings.
- **Shoot hibernation:** Hibernation overrides preservation. All machines are scaled down when a Shoot is hibernated.
- **Race condition on `Failed`:** If a machine is annotated for preservation at the moment it enters `Failed` phase, MCM may not act on the annotation before the machine is terminated. For higher reliability, annotate the machine or node when the node is observed as `NotReady` or the machine is in `Unknown` phase, before the machine transitions fully to `Failed`.
- **Race condition with CA:** If CA initiates a scale-down before MCM can apply the `scale-down-disabled` annotation as part of preservation, the machine may be removed before preservation takes effect.
- **Kubernetes version:** Preservation of backing machines for nodes that never registered (unregistered nodes) requires Kubernetes >= v1.34.

## Viewing Preservation Status

Preservation state is visible in two places:

**On the Node object** — a `NodeCondition` of type `Preserved` is set:

| Reason | Meaning |
|---|---|
| `Preserved by MCM.` | The node is auto-preserved |
| `Preserved by user.` | The node is manually preserved |
| `Preservation stopped.` | Preservation has ended |

**On the Machine object** — `status.currentStatus.preserveExpiryTime` shows when preservation will end.

**In the Shoot status** — the `NoPreservedFailedMachines` condition (see above) reflects whether any failed machines are currently preserved across the cluster.
