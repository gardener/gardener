---
title: Workerless `Shoot`s
description: What is a Workerless Shoot and how to create one
---

# Workerless `Shoot`s

Starting from `v1.71`, users can create a `Shoot` without any workers, known as a "workerless `Shoot`". Previously, worker nodes had to always be included even if users only needed the Kubernetes control plane. With workerless `Shoot`s, Gardener will not create any worker nodes or anything related to them.

Here's an example manifest for a local workerless `Shoot`:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: local
  namespace: garden-local
spec:
  cloudProfile:
    name: local
  region: local
  provider:
    type: local
  kubernetes:
    version: 1.31.1
```

> :warning: It's important to note that a workerless `Shoot` cannot be converted to a `Shoot` with workers or vice versa.

As part of the control plane, the following components are deployed in the seed cluster for workerless `Shoot`:
 - etcds
 - kube-apiserver
 - kube-controller-manager
 - gardener-resource-manager
 - logging and monitoring components
 - extension components (if they support workerless `Shoot`s, see [here](../../extensions/resources/extension.md#what-is-required-to-register-and-support-an-extension-type))
