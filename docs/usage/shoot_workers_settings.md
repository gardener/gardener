# Configure all shoot workers

Users can configure settings, that affect all worker nodes, in the `Shoot` API via `.spec.provider.workersSettings`.

Currently, we have the following fields in `WorkersSettings`:

## EnsureSSHAccessDisabled

`EnsureSSHAccessDisabled` indicates whether the worker nodes `sshd.service` is disabled. When set to `true` a service `sshddisabler.service` is started in each worker which runs a script every 15 seconds to stop and disable `sshd.service`, it also tereminates all already established ssh connections.

`EnsureSSHAccessDisabled` is set to `false` by default and when it is disabled it does not guarantee that `sshd.service` is enabled. If this configuration is set to `true` then edited to `false` the `sshd.service` will not be enabled automatically even if it is enabled by default for the specific `os`.

### Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workersSettings:
      ensureSSHAccessDisabled: true
```