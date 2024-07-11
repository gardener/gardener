---
title: Shoot Worker Nodes Settings
description: Configuring SSH Access through '.spec.provider.workersSettings`
---

# Shoot Worker Nodes Settings

Users can configure settings affecting all worker nodes via `.spec.provider.workersSettings` in the `Shoot` resource.

## SSH Access

`SSHAccess` indicates whether the `sshd.service` should be running on the worker nodes. This is ensured by a systemd service called `sshd-ensurer.service` which runs every 15 seconds on each worker node. When set to `true`, the systemd service ensures that the `sshd.service` is enabled and running. If it is set to `false`, the systemd service ensures that `sshd.service` is stopped and disabled. This also terminates all established SSH connections. In addition, when this value is set to `false`, existing `Bastion` resources are deleted during `Shoot` reconciliation and new ones are prevented from being created, SSH keypairs are not created/rotated, SSH keypair secrets are deleted from the Garden cluster, and the `gardener-user.service` is not deployed to the worker nodes.

`sshAccess.enabled` is set to `true` by default.

### Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workersSettings:
      sshAccess:
        enabled: false
```
