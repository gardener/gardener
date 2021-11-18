# Controlling the Kubernetes versions for specific worker pools

Since Gardener `v1.36`, worker pools can have different Kubernetes versions specified than the control plane.

It must be enabled by setting the featureGate `WorkerPoolKubernetesVersion: true` in the gardenlet's component configuration.

In earlier Gardener versions all worker pools inherited the Kubernetes version of the control plane. Once the Kubernetes version of the control plane was modified, all worker pools have been updated as well (either by rolling the nodes in case of a minor version change, or in-place for patch version changes).

In order to gracefully perform Kubernetes upgrades (triggering a rolling update of the nodes) with workloads sensitive to restarts (e.g., those dealing with lots of data), it might be required to be able to gradually perform the upgrade process.
In such cases, the Kubernetes version for the worker pools can be pinned (`.spec.provider.workers[].kubernetes.version`) while the control plane Kubernetes version (`.spec.kubernetes.version`) is updated.
This results in the nodes being untouched while the control plane is upgraded. 
Now a new worker pool (with the version equal to the control plane version) can be added.
Administrators can then reschedule their workloads to the new worekr pool according to their upgrade requirements and processes.

## Example Usage in a `Shoot`

```yaml
spec:
  kubernetes:
    version: 1.20.1
  provider:
    workers:
    - name: data1
      kubernetes:
        version: 1.19.1
    - name: data2
```

- If `.kubernetes.version` is not specified in a worker pool then the Kubernetes version of the kubelet is inherited from the control plane (`.spec.kubernetes.version`), i.e., in above example the `data2` pool will use `1.20.1`.
- If `.kubernetes.version` is specified in a worker pool then it must meet the following constraints:
  - It must be at most two minor versions lower than the control plane version.
  - If it was not specified before then no downgrade is possible (you cannot set it to `1.19.1` while `.spec.kubernetes.version` is already `1.20.1`). The "two minor version skew" is only possible if the worker pool version is set to control plane version and then the control plane was updated gradually two minor versions.
  - If the version is removed from the worker pool, only one minor version difference is allowed to the control plane (you cannot upgrade a pool from version `1.18.0` to `1.20.0` in one go).

Automatic updates of Kubernetes versions (see [this document](shoot_maintenance.md#automatic-version-updates)) also apply to worker pool Kubernetes versions.
