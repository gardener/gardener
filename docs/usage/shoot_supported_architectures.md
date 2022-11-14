# Supported CPU Architectures for Shoot Worker Nodes

Users can create shoot clusters with worker groups having virtual machines of different architectures. CPU architecture of each worker pool can be specified in the `Shoot` specification as follows:-

## Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      machine:
        architecture: <some-cpu-architecture>
```

If no value is specified for the architecture field it defaults to `amd64`. For a valid shoot object, a machine should be present in the respective CloudProfile with the same CPU architecture as specified in Shoot yaml.

Currently, Gardener supports two most widely used CPU architectures:-

* `amd64`
* `arm64`
