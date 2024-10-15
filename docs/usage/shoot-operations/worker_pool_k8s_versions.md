---
title: Controlling the Kubernetes Versions for Specific Worker Pools
---

# Controlling the Kubernetes Versions for Specific Worker Pools

Since Gardener `v1.36`, worker pools can have different Kubernetes versions specified than the control plane.

In earlier Gardener versions, all worker pools inherited the Kubernetes version of the control plane. Once the Kubernetes version of the control plane was modified, all worker pools have been updated as well (either by rolling the nodes in case of a minor version change, or in-place for patch version changes).

In order to gracefully perform Kubernetes upgrades (triggering a rolling update of the nodes) with workloads sensitive to restarts (e.g., those dealing with lots of data), it might be required to be able to gradually perform the upgrade process.
In such cases, the Kubernetes version for the worker pools can be pinned (`.spec.provider.workers[].kubernetes.version`) while the control plane Kubernetes version (`.spec.kubernetes.version`) is updated.
This results in the nodes being untouched while the control plane is upgraded. 
Now a new worker pool (with the version equal to the control plane version) can be added.
Administrators can then reschedule their workloads to the new worker pool according to their upgrade requirements and processes.

## Example Usage in a `Shoot`

```yaml
spec:
  kubernetes:
    version: 1.27.4
  provider:
    workers:
    - name: data1
      kubernetes:
        version: 1.26.8
    - name: data2
```

- If `.kubernetes.version` is not specified in a worker pool, then the Kubernetes version of the kubelet is inherited from the control plane (`.spec.kubernetes.version`), i.e., in the above example, the `data2` pool will use `1.26.8`.
- If `.kubernetes.version` is specified in a worker pool, then it must meet the following constraints:
  - It must be at most two minor versions lower than the control plane version.
  - If it was not specified before, then no downgrade is possible (you cannot set it to `1.26.8` while `.spec.kubernetes.version` is already `1.27.4`). The "two minor version skew" is only possible if the worker pool version is set to the control plane version and then the control plane was updated gradually by two minor versions.
  - If the version is removed from the worker pool, only one minor version difference is allowed to the control plane (you cannot upgrade a pool from version `1.25.0` to `1.27.0` in one go).

Automatic updates of Kubernetes versions (see [Shoot Maintenance](../shoot/shoot_maintenance.md#automatic-version-updates)) also apply to worker pool Kubernetes versions.
