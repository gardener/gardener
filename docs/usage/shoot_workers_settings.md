# Configure all shoot workers

Users can configure settings, that affect all worker nodes, in the `Shoot` API via `.spec.provider.workersSettings`.

## EnsureSSHAccessDisabled

`EnsureSSHAccessDisabled` indicates whether the `sshd.service` on the worker nodes should be disabled. When set to `true` a service called `sshddisabler.service` is started on each worker which runs a script every 15 seconds in order to ensure that the `sshd.service` is stopped and disabled. It also terminates all established ssh connections.

`EnsureSSHAccessDisabled` is set to `false` by default and when it is disabled it does not guarantee that `sshd.service` is enabled or running. If this configuration is set to `true` then edited to `false` the `sshd.service` will not be enabled automatically even if it is enabled by default for the specific operating system.

### Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workersSettings:
      ensureSSHAccessDisabled: true
```
