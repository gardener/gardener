---
title: Default Seccomp Profile
description: Enable the use of `RuntimeDefault` as the default seccomp profile through `spec.kubernetes.kubelet.seccompDefault`
---

# Default Seccomp Profile and Configuration 

This is a short guide describing how to enable the defaulting of seccomp profiles for Gardener managed workloads in the seed. Running pods in `Unconfined` (seccomp disabled) mode is undesirable since this is the least restrictive profile. Also, mind that any privileged container will always run as `Unconfined`. More information about seccomp can be found in this [Kubernetes tutorial](https://kubernetes.io/docs/tutorials/security/seccomp/).

## Setting the Seccomp Profile to RuntimeDefault for Shoot Clusters

You can enable the use of `RuntimeDefault` as the default seccomp profile for all workloads. If enabled, the kubelet will use the `RuntimeDefault` seccomp profile by default, which is defined by the container runtime, instead of using the `Unconfined` mode. More information for this feature can be found in the [Kubernetes documentation](https://kubernetes.io/docs/tutorials/security/seccomp/#enable-the-use-of-runtimedefault-as-the-default-seccomp-profile-for-all-workloads).

To use seccomp profile defaulting, you must run the kubelet with the `SeccompDefault` feature gate enabled (this is the default).

### How to Configure

To enable this feature, the kubelet `seccompDefault` configuration parameter must be set to `true` in the shoot's spec.

```yaml
spec:
  kubernetes:
    version: 1.25.0
    kubelet:
      seccompDefault: true
```

Please refer to the examples in this [yaml file](../../../example/90-shoot.yaml) for more information.

> **Note:** Please note that this feature is still in Alpha, so you might see instabilities every now and then. 