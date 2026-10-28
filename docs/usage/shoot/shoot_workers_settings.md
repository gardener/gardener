---
title: Shoot Worker Nodes Settings
description: Configuring SSH Access and sysctls through '.spec.provider.workersSettings`
---

# Shoot Worker Nodes Settings

Users can configure settings affecting all worker nodes via `.spec.provider.workersSettings` in the `Shoot` resource.

## SSH Access

`SSHAccess` indicates whether the `sshd.service` should be running on the worker nodes. This is ensured by a systemd service called `sshd-ensurer.service` which runs every 15 seconds on each worker node. When set to `true`, the systemd service ensures that the `sshd.service` is unmasked, enabled and running. If it is set to `false`, the systemd service ensures that `sshd.service` is disabled, masked and stopped. This also terminates all established SSH connections on the host. In addition, when this value is set to `false`, existing `Bastion` resources are deleted during `Shoot` reconciliation and new ones are prevented from being created, SSH keypairs are not created/rotated, SSH keypair secrets are deleted from the Garden cluster, and the `gardener-user.service` is not deployed to the worker nodes.

`sshAccess.enabled` is set to `true` by default.

### Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workersSettings:
      sshAccess:
        enabled: false
```

## Sysctls

`sysctls` is a map of kernel settings that is applied to all worker pools, so the same settings do not have to be repeated in every `.spec.provider.workers[].sysctls` entry.

A worker pool can still define its own `.spec.provider.workers[].sysctls`. If it does, the per-pool value **fully overrides** the global one for that pool — the two maps are **not merged** (this follows the same pattern as the kubelet configuration, where `.spec.provider.workers[].kubernetes.kubelet` overrides `.spec.kubernetes.kubelet`). Worker pools that do not set their own `sysctls` inherit the global value.

### Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workersSettings:
      sysctls:
        net.ipv4.tcp_wmem: "4096 131072 16777216"
        net.ipv4.tcp_rmem: "4096 131072 16777216"
    workers:
    - name: pool-with-override
      # This pool ignores the global sysctls above and uses only these.
      sysctls:
        vm.swappiness: "10"
    - name: pool-inheriting-global
      # This pool has no sysctls of its own and inherits the global settings.
```
