# Gardener extensibility to support shoot additional container runtimes

## Table of Contents

* [Summary](#summary)
* [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
* [Proposal](#proposal)
* [Design Details](#design-details)
* [Alternatives](#alternatives)

## Summary

Gardener-managed Kubernetes clusters are sometimes used to run sensitive workloads, which sometimes are comprised of OCI images originating from untrusted sources. Additional use-cases want to leverage economy-of-scale to run workloads for multiple tenants on the same cluster.  In some cases, Gardener users want to use operating systems which do not easily support the Docker engine.

This proposal aims to allow Gardener Shoot clusters to use CRI instead of the legacy Docker API, and to provide extension type for adding CRI shims (like [GVisor](https://gvisor.dev/) and [Kata Containers](https://katacontainers.io/)) which can be used to add support in Gardener Shoot clusters for these runtimes.

## Motivation

While pods and containers are intended to create isolated areas for concurrently running workloads on nodes, this isolation is not as robust as could be expected. Containers leverage the core Linux CGroup and Namespace features to isolate workloads, and many kernel vulnerabilities have the potential to allow processes to escape from their isolation. Once a process has escaped from its container, any other process running on the same node is compromised. Several projects try to mitigate this problem; for example Kata Containers allow isolating a Kubernetes Pod in a micro-vm, gVisor reduces the kernel attack surface by adding another level of indirection between the actual payload and the real kernel.

Kubernetes supports running pods using these alternate runtimes via the [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) concept, which was promoted to Beta in Kubernetes 1.14. Once Kubernetes is configured to use the Container Runtime Interface to control pods, it becomes possible to leverage CRI and run specific pods using different Runtime Classes. Additionally, configuring Kubernetes to use CRI instead of the legacy Dockershim is [faster](https://events19.linuxfoundation.org/wp-content/uploads/2017/11/How-Container-Runtime-Matters-in-Kubernetes_-OSS-Kunal-Kushwaha.pdf).

The motivation behind this proposal is to make all of this functionality accessible to Shoot clusters managed by Gardener.

### Goals

* Gardener must allow to configue its managed clusters with the CRI interface instead of the legacy Dockershim.
* Low-level runtimes like gVisor or Kata Containers are provided as gardener extensions which are (optionally) installed into a landscape by the Gardener operator. There must be no runtime-specific knowledge in the core Gardener code.
* It shall be possible to configure multiple low-level runtimes in Shoot clusters, on the Worker Group level.

## Proposal

Gardener today assumes that all supported operating systems have Docker pre-installed in the base image. Starting with Docker Engine 1.11, Docker itself was [refactored](https://www.docker.com/blog/docker-engine-1-11-runc/) and cleaned-up to be based on the [containerd](https://containerd.io/) library. The first phase would be to allow the change of the Kubelet configuration as described [here](https://kubernetes.io/docs/setup/production-environment/container-runtimes/#containerd) so that Kubernetes would use containerd instead of the default Dockershim. This will be implemented for CoreOS, Ubuntu, and SuSE-CHost.

We will implement two Gardener extensions, providing gVisor and Kata Containers as options for Gardener landscapes.
The `WorkerGroup` specification will be extended to allow specifying the CRI name and a list of additional required Runtimes for nodes in that group. For example:
```yaml
workers:
- name: worker-b8jg5
  machineType: m5.large
  volumeType: gp2
  volumeSize: 50Gi
  autoScalerMin: 1
  autoScalerMax: 2
  maxSurge: 1
  cri:
    name: containerd
    containerRuntimes:
    - type: gvisor
    - type: kata-containers
  machineImage:
    name: coreos
    version: 2135.6.0
```

Each extension will need to address the following concern:

1. Add the low-level runtime binaries to the worker nodes. Each extension should get the runtime binaries from a container.
1. Hook the runtime binary into the containerd configuration file, so that the runtime becomes available to containerd.
1. Apply a label to each node that allows identifying nodes where the runtime is available.
1. Apply the relevant `RuntimeClass` to the Shoot cluster, to expose the functionality to users.
1. Provide a separate binary with a `ValidatingWebhook` (deployable to the garden cluster) to catch invalid configurations. For example, Kata Containers on AWS requires a `machineType` of `i3.metal`, so any `Shoot` requests with a Kata Containers runtime and a different machine type on AWS should be rejected.

## Design Details

1. Change the nodes container runtime to work with CRI and ContainerD (Only if specified in the Shoot spec):
    1. In order to configure each worker machine in the cluster to work with CRI, the following configurations should be done:
        1. Add kubelet execution flags:
            1. --container-runtime=remote
            2. --container-runtime-endpoint=unix:///run/containerd/containerd.sock
        2. Make sure that default containerd configuration file exist in path /etc/containerd/config.toml.

    2. ContainerD and Docker configurations are different for each OS. To make sure the default configurations above works well in each worker machine, each OS extension would be responsible to configure them during the reconciliation of the
       OperatingSystemConfig:
        1. os-ubuntu -
            1. Create ContainerD unit Drop-In to execute ContainerD with the default configurations file in path /etc/containerd/config.toml.
            2. Create the container runtime metadata file with a OS path for binaries installations: /usr/bin.
        2. os-coreos -
            1. Create ContainerD unit Drop-In to execute ContainerD with the default configurations file in path /etc/containerd/config.toml.
            2. Create Docker Drop-In unit to execute Docker with the correct socket path of ContainerD.
            3. Create the container runtime metadata file with a OS path for binaries installations: /var/bin.
        3. os-suse-chost -
           1. Create ContainerD service unit and execute ContainerD with the default configurations file in path /etc/containerd/config.toml.
           2. Download and install ctr-cli which is not shipped with the current SuSe image.
           3. Create the container runtime metadata file with a OS path for binaries installations /usr/sbin.

    3. To rotate the ContainerD (CRI) logs we will activate the kubelet feature flag: CRIContainerLogRotation=true.
    4. Docker monitor service will be replaced with equivalent ContainerD monitor service.

3. Validate workers additional runtime configurations:
   1. Disallow additional runtimes with shoots < 1.14
   2. kata-container validation: Machine type support nested virtualization.
4. Add support for each additional container runtime in the cluster.
    1. In order to install each additional available runtime in the cluster we should:
        1. Install the runtime binaries in each Worker's pool nodes that specified the runtime support.
        2. Apply the relevant RuntimeClass to the cluster.

    2. The installation above should be done by a new kind of extension: ContainerRuntime resource. For each container runtime type (Kata-container/gvisor) a dedicate extension controller will be created.
        1. A label for each container runtime support will be added to every node that belongs to the worker pool. This should be done similar
           to the way labels created today for each node, through kubelet execution parameters (_kubelet.flags: --node-labels). When creating the OperatingSystemConfig (original) for the worker each container runtime support should be mapped to a label on the node.
           For Example:
           label: container.runtime.kata-containers=true (shoot.spec.cloud.<IAAS>.worker.containerRuntimes.kata-container)
           label: container.runtime.gvisor=true (shoot.spec.cloud.<IAAS>.worker.containerRuntimes.gvisor)
        2. During the Shoot reconciliation (Similar steps to the Extensions today) Gardener will create new ContainerRuntime resource if a container runtime exist in at least one worker spec:
            ```yaml
            apiVersion: extensions.gardener.cloud/v1alpha1
            kind: ContainerRuntime
            metadata:
              name: kata-containers-runtime-extention
              namespace: shoot--foo--bar
            spec:
              type: kata-containers
            ```

            Gardener will wait that all ContainerRuntimes extensions will be reconciled by the appropriate extensions controllers.
        3. Each runtime extension controller will be responsible to reconcile it's RuntimeContainer resource type.
           rc-kata-containers extension controller will reconcile RuntimeContainer resource from type kata-container and rc-gvisor will reconcile RuntimeContainer resource from gvisor.
           Reconciliation process by container runtime extension controllers:
           1. Runtime extension controller from specific type should apply a chart which responsible for the installation of the runtime container in the cluster:
                1. DaemonSet which will run a privileged pod on each node with the label: container.runtime.<type of the resource>:true The pod will be responsible for:
                    1. Copy the runtime container binaries (From extension package ) to the relevant path in the host OS.
                    2. Add the relevant container runtime plugin section to the containerd configuration file (/etc/containerd/config.toml).
                    3. Restart containerd in the node.
                3. RuntimeClasses in the cluster to support the runtime class. for example:
                   ```yaml
                   apiVersion: node.k8s.io/v1beta1
                   kind: RuntimeClass
                   metadata:
                     name: gvisor
                   handler: runsc
                   ```
           2. Update the status of the relevant RuntimeContainer resource to succeeded.

-->

## Alternatives
