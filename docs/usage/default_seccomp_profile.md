---
title: Default Seccomp Profile
---

# Default Seccomp Profile and Configuration 

This is a short guide describing how to enable the defaulting of seccomp profiles for Gardener managed workloads in the seed.

## Default Kubernetes Behavior

The state of Kubernetes in versions < 1.25 is such that all workloads by default run in `Unconfined` (seccomp disabled) mode. This is undesirable since this is the least restrictive profile. Also mind that any privileged container will always run as `Unconfined`. More information about seccomp can be found in this [Kubernetes tutorial](https://kubernetes.io/docs/tutorials/security/seccomp/).

## Setting the Seccomp Profile to RuntimeDefault

To address the above issue, Gardener provides a webhook that is capable of mutating pods in the seed clusters, explicitly providing them with a seccomp profile type of `RuntimeDefault`. This profile is defined by the container runtime and represents a set of default syscalls that are allowed or not.
```yaml
spec:
  securityContext:
    seccompProfile:
      type: RuntimeDefault
```

A `Pod` is mutated when all of the following preconditions are fulfilled:
1. The `Pod` is created in Gardener managed namespace.
2. The `Pod` is NOT labeled with `seccompprofile.resources.gardener.cloud/skip`.
3. The `Pod` does NOT explicitly specify `.spec.securityContext.seccompProfile.type`.

### How to Configure

To enable the usage this feature, the Gardenlet `DefaultSeccompProfile` feature gate must be set to `true`.

```yaml
featureGates:
  DefaultSeccompProfile: true
``` 
Please refer to the examples [here](../../example/20-componentconfig-gardenlet.yaml) for more information.

Once the feature gate is enabled, the webhook will be registered and configured for the seed cluster. Newly created pods will be mutated to have their seccomp profile set to `RuntimeDefault`.

> Please note that this feature is still in Alpha, so you might see instabilities every now and then. 

## Future steps

The Gardener team plans to provide support for a similar feature for shoot clusters by enabling the `kubelet`'s `SeccompDefault` feature gate. This will happen in a future release once Kubernetes promotes the `SeccompDefault` feature gate to Beta (expected in version 1.25). See [here](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) for more information on Kubernetes feature gates. Once this support is implemented the `Gardenlet`'s `DefaultSeccompProfile` feature gate may become obsolete in scenarios where the `kubelet`'s `SeccompDefault` feature gate is enabled, i.e. for ManagedSeeds running Kubernetes version >= 1.25.
