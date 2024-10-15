# Supported CPU Architectures for Shoot Worker Nodes

Users can create shoot clusters with worker groups having virtual machines of different architectures. CPU architecture of each worker pool can be specified in the `Shoot` specification as follows:

## Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      machine:
        architecture: <some-cpu-architecture> # optional
```

If no value is specified for the architecture field, it defaults to `amd64`. For a valid shoot object, a machine type should be present in the respective `CloudProfile` with the same CPU architecture as specified in the `Shoot` yaml. Also, a valid machine image should be present in the `CloudProfile` that supports the required architecture specified in the `Shoot` worker pool.

## Example Usage in a `CloudProfile`

```yaml
spec:
  machineImages:
  - name: test-image
    versions:
    - architectures: # optional
      - <architecture-1>
      - <architecture-2>
      version: 1.2.3
  machineTypes:
  - architecture: <some-cpu-architecture>
    cpu: "2"
    gpu: "0"
    memory: 8Gi
    name: test-machine
```

Currently, Gardener supports two of the most widely used CPU architectures:

* `amd64`
* `arm64`
