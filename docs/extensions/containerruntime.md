---
title: ContainerRuntime
---

# Gardener Container Runtime Extension

At the lowest layers of a Kubernetes node is the software that, among other things, starts and stops containers. It is called “Container Runtime”.
The most widely known container runtime is Docker, but it is not alone in this space. In fact, the container runtime space has been rapidly evolving.

Kubernetes supports different container runtimes using Container Runtime Interface (CRI) – a plugin interface which enables kubelet to use a wide variety of container runtimes.

Gardener supports creation of Worker machines using CRI. For more information, see [CRI Support](operatingsystemconfig.md#cri-support).

## Motivation

Prior to the `Container Runtime Extensibility` concept, Gardener used Docker as the only
container runtime to use in shoot worker machines. Because of the wide variety of different container runtimes
offering multiple important features (for example, enhanced security concepts), it is important to enable end users to use other container runtimes as well.

## The `ContainerRuntime` Extension Resource

Here is what a typical `ContainerRuntime` resource would look like:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: ContainerRuntime
metadata:
  name: my-container-runtime
spec:
  binaryPath: /var/bin/containerruntimes
  type: gvisor
  workerPool:
    name: worker-ubuntu
    selector:
      matchLabels:
        worker.gardener.cloud/pool: worker-ubuntu
```

Gardener deploys one `ContainerRuntime` resource per worker pool per CRI.
To exemplify this, consider a Shoot having two worker pools (`worker-one`, `worker-two`) using `containerd` as the CRI as well as `gvisor` and `kata` as enabled container runtimes.
Gardener would deploy four `ContainerRuntime` resources. For `worker-one`: one `ContainerRuntime` for type `gvisor` and one for type `kata`. The same resource are being deployed for `worker-two`.

## Supporting a New Container Runtime Provider

To add support for another container runtime (e.g., gvisor, kata-containers), a container runtime extension controller needs to be implemented. It should support Gardener's supported CRI plugins.

The container runtime extension should install the necessary resources into the shoot cluster (e.g., `RuntimeClass`es), and it should copy the runtime binaries to the relevant worker machines in path: `spec.binaryPath`. 
Gardener labels the shoot nodes according to the CRI configured: `worker.gardener.cloud/cri-name=<value>` (e.g `worker.gardener.cloud/cri-name=containerd`) and multiple labels for each of the container runtimes configured for the shoot Worker machine:
`containerruntime.worker.gardener.cloud/<container-runtime-type-value>=true` (e.g `containerruntime.worker.gardener.cloud/gvisor=true`).
The way to install the binaries is by creating a daemon set which copies the binaries from an image in a docker registry to the relevant labeled Worker's nodes (avoid downloading binaries from the internet to also cater with isolated environments).

For additional reference, please have a look at the [runtime-gvsior](https://github.com/gardener/gardener-extension-runtime-gvisor) provider extension, which provides more information on how to configure the necessary charts, as well as the actuators required to reconcile container runtime inside the `Shoot` cluster to the desired state.
