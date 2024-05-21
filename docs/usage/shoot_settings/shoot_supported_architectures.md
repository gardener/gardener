---
weight: 5
title: Shoot Workers
description: How to specify a shoot worker group given the respective `CloudProfile` 
---

# Configuring a shoot worker group 

Users can create shoot clusters with worker groups having virtual machines of different types and with different images. For a valid shoot object, a machine type and image should be present in the respective `CloudProfile` that support the required specification in the `Shoot` yaml.

## Supported CPU Architectures for Shoot Worker Nodes

Currently, Gardener supports two of the most widely used CPU architectures:

* `amd64`
* `arm64`

If no value is specified for the architecture field, it defaults to `amd64`.

## Example `CloudProfile`

```yaml
spec:
  machineImages:
  - name: test-image
    versions:
    - architectures:
      - amd64
      - arm64
      version: 1.2.3
  machineTypes:
  - architecture: amd64
    cpu: "2"
    gpu: "0"
    memory: 8Gi
    name: test-machine
```

## Example Usage in a `Shoot`

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      machine:
        architecture: amd64
        type: test-machine
        image:
          name: test-image
          version: 1.2.3
```

