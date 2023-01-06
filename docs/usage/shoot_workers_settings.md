# Shoot Worker Nodes Settings

Users can configure settings affecting all worker nodes via `.spec.provider.workersSettings` in the `Shoot` resource.

## SSH Access

`SSHAccess` indicates whether the `sshd.service` on the worker nodes should be be enabled and running or disabled and stopped . When set to `true`, a systemd service called `sshd-ensurer.service` is started on each worker node. It runs a script every 15 seconds in order to ensure that the `sshd.service` is enabled and running. If it is set to `false`, the `sshd-ensurer.service` service ensures that `sshd.service` is stopped and disabled. It also terminates all established SSH connections, existing `Bastion` resources are deleted in shoot reconcilation and new ones are prevented from being created. SSH keypairs stopped being rotated, secrets with suffixes "ssh-keypair" and "ssh-keypair.old" are deleted and the gardener-user.service is not deployed to worker nodes.

`SSHAccess` is set to `true` by default.

### Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workersSettings:
      sshAccess:
        enabled: false
```
