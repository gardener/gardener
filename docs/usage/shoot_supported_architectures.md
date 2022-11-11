# Shoot Supported Architectures

Users can create shoot with worker groups having virtual machines of different architectures. CPU architecture of each worker pool can be specified in shoot yaml as follows:-

## Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      machine:
        architecture: <some-cpu-architecture>
```

If there is no value specified for architecture it defaults to `amd64`.

Currently, Gardener supports two most widely used CPU architectures:-

* `amd64`
* `arm64`
