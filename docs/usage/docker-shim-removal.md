# Kubernetes dockershim Removal

## What's Happening?
With Kubernetes v1.20, the built-in dockershim [was deprecated](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.20.md#dockershim-deprecation) and is scheduled to be [removed with v1.24](https://github.com/kubernetes/enhancements/issues/2221).
Don't Panic! The Kubernetes community has [published a blogpost](https://kubernetes.io/blog/2020/12/02/dont-panic-kubernetes-and-docker/) and an [FAQ](https://kubernetes.io/blog/2022/02/17/dockershim-faq/) with more information.

Gardener also needs to switch from using the built-in dockershim to `containerd`.
Gardener will not change already running Shoot clusters. But changes to the container runtime will be coupled to the K8s version selected by the Shoot:
- Starting with K8s version 1.22, Shoots not explicitly selecting a container runtime will get `containerd` instead of `docker`. Shoots can still select `docker` explicitly if needed.
- Starting with K8s version 1.23 `docker` can no longer be selected.

At this point in time, we have no plans to support other container runtimes, such as `cri-o`.

## What Should I Do?
As a Gardener operator:
- Add `containerd` and `docker` to `.spec.machineImages[].versions[].cri.name` in your CloudProfile to allow users selecting a container runtime for their Shoots (see below). 
> **Note:** Please take a look at our detailed information regarding [container runtime support in Gardener Operating System Extensions](#container-runtime-support-in-gardener-operating-system-extensions).
- Update your cloud provider extensions to avoid a node rollout when a Shoot is configured from `cri: nil` to `cri.name: docker`. 
> **Note:** Please take a look at our detailed information regarding [stable Worker node hash support in Gardener Provider Extensions](#stable-worker-node-hash-support-in-gardener-provider-extensions).

As a shoot owner:
- [Check if you have dependencies to the `docker` container runtime](https://kubernetes.io/docs/tasks/administer-cluster/migrating-from-dockershim/check-if-dockershim-deprecation-affects-you/#find-docker-dependencies). 
> **Note:** This is not only about your actual workload, but also concerns ops tooling as well as logging, monitoring, and metric agents installed on the nodes.
- Test with `containerd`:
  - Create a new Shoot or add a Worker Pool to an existing one.
  - [Set `.spec.provider.workers[].cri.name: containerd`](../api-reference/core.md#cri) for your Shoot.
- Once testing is successful, switch to `containerd` with your production workload. You don't need to wait for kubernetes v1.22, `containerd` is considered production ready as of today.
- If you find dependencies to `docker`, set `.spec.provider.workers[].cri.name: docker` explicitly to avoid defaulting to `containerd` once you update your Shoot to kubernetes v1.22.

## Timeline
  - **2021-08-04:** Kubernetes v1.22 released. Shoots using this version get `containerd` as default container runtime. Shoots can still select `docker` explicitly if needed.
  - **2021-12-07:** Kubernetes v1.23 released. Shoots using this version can no longer select `docker` as container runtime.
  - **2022-06-28:** Kubernetes v1.21 goes out of maintenance. This is the last version not affected by these changes. Make sure you have tested thoroughly and set the correct configuration for your Shoots!
  - **2022-10-28:** Kubernetes v1.22 goes out of maintenance. This is the last version that you can use with `docker` as container runtime. Make sure you have removed any dependencies to `docker` as container runtime!

See [the official kubernetes documentation](https://kubernetes.io/releases/) for the exact dates for all releases.

## Container Runtime Support in Gardener Operating System Extensions

| Operating System | docker support     | containerd support |
|------------------|--------------------|--------------------|
| GardenLinux      | :white_check_mark: | >= v0.3.0 |
| Ubuntu           | :white_check_mark: | >= v1.4.0 |
| SuSE CHost       | :white_check_mark: | >= v1.14.0 |
| CoreOS/FlatCar   | :white_check_mark: | >= v1.8.0 |

> **Note**: If you're using a different Operating System Extension, start evaluating now if it provides support for `containerd`. Please refer to [our documentation of the `operatingsystemconfig` contract](https://github.com/gardener/gardener/blob/master/docs/extensions/operatingsystemconfig.md#cri-support) to understand how to support `containerd` for an Operating System Extension.

## Stable Worker Node Hash Support in Gardener Provider Extensions

Upgrade to these versions to avoid a node rollout when a Shoot is configured from `cri: nil` to `cri.name: docker`.

| Provider Extension | Stable worker hash support |
|--------------------|----------------------------|
| Alicloud           | >= 1.26.0                  |
| AWS                | >= 1.27.0                  |
| Azure              | >= 1.21.0                  |
| GCP                | >= 1.18.0                  |
| OpenStack          | >= 1.21.0                  |
| vSphere            | >= 0.11.0                  |

> **Note**: If you're using a different Provider Extension, start evaluating now if it keeps the worker hash stable when switching from `.spec.provider.workers[].cri: nil` to `.spec.provider.workers[].cri.name: docker`. This doesn't impact functional correctness, however, a node rollout will be triggered when users decide to configure `docker` for their shoots.
