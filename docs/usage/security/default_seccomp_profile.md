---
description: Enable the use of `RuntimeDefault` as the default seccomp profile through `spec.kubernetes.kubelet.seccompDefault`
---

# Default Seccomp Profile and Configuration 

This is a short guide describing how to enable the defaulting of seccomp profiles for Gardener managed workloads in the seed. Running pods in `Unconfined` (seccomp disabled) mode is undesirable since this is the least restrictive profile. Also, mind that any privileged container will always run as `Unconfined`. More information about seccomp can be found in this [Kubernetes tutorial](https://kubernetes.io/docs/tutorials/security/seccomp/).

## Setting the Seccomp Profile to RuntimeDefault for Seed Clusters

To address the above issue, Gardener provides a webhook that is capable of mutating pods in the seed clusters, explicitly providing them with a seccomp profile type of `RuntimeDefault`. This profile is defined by the container runtime and represents a set of default syscalls that are allowed or not.
```yaml
spec:
  securityContext:
    seccompProfile:
      type: RuntimeDefault
```

A `Pod` is mutated when all of the following preconditions are fulfilled:
1. The `Pod` is created in a Gardener managed namespace.
2. The `Pod` is NOT labeled with `seccompprofile.resources.gardener.cloud/skip`.
3. The `Pod` does NOT explicitly specify `.spec.securityContext.seccompProfile.type`.

### How to Configure

To enable this feature, the gardenlet `DefaultSeccompProfile` feature gate must be set to `true`.

```yaml
featureGates:
  DefaultSeccompProfile: true
``` 
Please refer to the examples in this [yaml file](../../../example/20-componentconfig-gardenlet.yaml) for more information.

Once the feature gate is enabled, the webhook will be registered and configured for the seed cluster. Newly created pods will be mutated to have their seccomp profile set to `RuntimeDefault`.

> **Note:** Please note that this feature is still in Alpha, so you might see instabilities every now and then. 

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
