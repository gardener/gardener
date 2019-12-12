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

This proposal aims to migrate Gardener clusters to use CRI by default instead of the legacy Docker API, and to provide extension type for adding CRI shims (like [GVisor](https://gvisor.dev/) and [Kata Containers](https://katacontainers.io/)) which can be used to add support in Gardener clusters for these runtimes.

## Motivation

While pods and containers are intended to create isolated areas for concurrently running workloads on nodes, this isolation is not as robust as could be expected. Containers leverage the core Linux CGroup and Namespace features to isolate workloads, and many kernel vulnerabilities have the potential to allow processes to escape from their isolation. Once a process has escaped from its container, any other process running on the same node is compromised. Several projects try to mitigate this problem; for example Kata Containers allow isolating a Kubernetes Pod in a micro-vm, gVisor reduces the kernel attack surface by adding another level of indirection between the actual payload and the real kernel.

Kubernetes supports running pods using these alternate runtimes via the [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) concept, which was promoted to Beta in Kubernetes 1.14. Once Kubernetes is configured to use the Container Runtime Interface to control pods, it becomes possible to leverage CRI and run specific pods using different Runtime Classes. Additionally, configuring Kubernetes to use CRI instead of the legacy Dockershim is [faster](https://events19.linuxfoundation.org/wp-content/uploads/2017/11/How-Container-Runtime-Matters-in-Kubernetes_-OSS-Kunal-Kushwaha.pdf).

The motivation behind this proposal is to make all of this functionality accessible to clusters managed by Gardener.

### Goals

* Gardener must default to configuring its managed clusters with the CRI interface instead of the legacy Dockershim. This change must be transparent to Gardener users and must simply work out-of-the-box for new and existing clusters.
* Low-level runtimes like gVisor or Kata Containers are provided as gardener extensions which are (optionally) installed into a landscape by the Gardener operator. There must be no runtime-specific knowledge in the core Gardener code.
* It shall be possible to configure multiple low-level runtimes in Gardener clusters, on the Worker Group level.

### Non-Goals

* Exposing the configuration of the underlying runtimes to the user is beyond the scope of this proposal. The initial version will support a default common-sense configuration for each of the underlying runtimes. If required, future enhancements will make it possible to fine-tune the underlying runtimes.

## Proposal

Gardener today assumes that all supported operating systems have Docker pre-installed in the base image. Starting with Docker Engine 1.11, Docker itself was [refactored](https://www.docker.com/blog/docker-engine-1-11-runc/) and cleaned-up to be based on the [containerd](https://containerd.io/) library. The first phase would be to change the default Kubelet configuration as described [here](https://kubernetes.io/docs/setup/production-environment/container-runtimes/#containerd) so that Kubernetes would use containerd instead of the default Dockershim. This will be implemented for CoreOS, Ubuntu, and SuSE-JeOS. 

Once Gardener clusters are migrated to use CRI, we will implement two Gardener extensions, providing gVisor and Kata Containers as options for Gardener landscapes. The `WorkerGroup` specification will be extended to allow specifying a list of additional required Runtimes for nodes in that group. For example:
```yaml
workers:
- name: worker-b8jg5
  machineType: m5.large
  volumeType: gp2
  volumeSize: 50Gi
  autoScalerMin: 1
  autoScalerMax: 2
  maxSurge: 1
  additionalRuntimes:
  - gvisor
  - kata-containers
  machineImage:
    name: coreos
    version: 2135.6.0
```

Each extension will need to address the following concern:

1. Add the low-level runtime binaries to the worker nodes. Each extension should store the runtime binary in a way that is accessible to a node to download and install during startup.
1. Hook the runtime binary into the containerd configuration file, so that the runtime becomes available to containerd.
1. Apply a label to each node that allows identifying nodes where the runtime is available.
1. Apply the relevant `RuntimeClass` to the Gardener cluster, to expose the functionality to users.
1. Provide a `ValidatingWebhook` to catch invalid configurations. For example, Kata Containers on AWS requires a `machineType` of `i3.metal`.

Since each operating system distribution has different methods of installing software (apt on ubuntu, zypper on SuSE, Torcx on CoreOS/Flatcar), we will enhance the operating system extensions to add a standard `installSoftware` script which can abstract the mechanics of installing new software on a node. The new runtime extensions will use this new functionality to install the relevant runtimes to the worker nodes.

## Design Details

1. In order to configure each shoot in the cluster to work with CRI the following configurations should be done:
    1. Add kubelet execution flags (Defined in the OperationSystemConfig - original):
        1. --container-runtime=remote
        2. --container-runtime-endpoint=unix:///.../containerd.socket
    2. Make sure that containerd is configured to enable the CRI plugin. In the containerd config.toml "cri" plugin must not be disabled.
2. Containerd socket path and containerd configurations are different for each OS. To make sure the configurations above are set in each shoot, each OS extension would be responsible to configure them.
    1. os-ubuntu - 
        1. Add a controlplane webhook to manipulate the OperatingSystemConfig and add the following flags to the kubelet execution command:
            1. --container-runtime=remote
            2. --container-runtime-endpoint=unix:///run/containerd/containerd.sock
        2. If there will be a config.toml file in /etc/containerd/config.toml, containerd service will start with this file configuration by default.
    2. os-coreos - 
        1. Add a controlplane webhook to manipulate the OperatingSystemConfig and add the following flags to the kubelet execution command:
           1. --container-runtime=remote
           2. --container-runtime-endpoint=unix:///var/run/docker/libcontainerd/docker-containerd.socket
        2. Add a generator that will override the default containerd.service unit. It will create a new unit in:  /etc/systemd/system/containerd.service  unit that will start the containerd service with a configuration file path: /etc/containerd/config.toml
    3. os-suse-jeos - __TBD__ 
3. In order to install additional runtimes on the machine we must provide the installation scripts to the machine (Shoot) file system and run them.
   The installation script should download and install the runtime binaries in each OS.
   Instructions for Gardener to copy files to the Shoot filesystem and execute them is done via the cloud config secret.
   
   *Consider installing by default all additional runtimes available (Binaries and containerd plugin configurations). The cluster configuration installation will be done only if these additional runtimes were declared) It is also possible to install only the declared runtimes (Maybe save them in the OperatingSystemConfig from the shoot CRD __TBD__)*
   Each OS extention should implement a generator that will:
    1. copy the installation script file.
       ```yaml
       #cloud-config
       write_files:
         path: '/var/lib/additional-runtime/install.sh'
         ...
       ```
    2. create a service unit to run it.
       ```yaml
       - path: '/etc/systemd/system/gardener-user.service'
         encoding: b64
         content: 
           [Unit]
           Description=Download and install additional runtimes binaries
           ...
       
           [Service]
           ExecStart=/var/lib/additional-runtime/install.sh
           ...
       ```

    3. configure containerd (config.toml) to work with the additional runtime shim. Copy the config.toml file to /etc/containerd/config.toml
        ```yaml
          #cloud-config
          write_files:
            path: '/etc/containerd/config.toml'
            ...
          ```
   Specific installations of container runtimes in each OS __TBD__
           
4. Installation of the runtime environment in the cluster is done by applying the RuntimeClass. This should be done only
   if the user chose the additional runtime in the Shoot specification. This should be done similar to the deployment of other addons into the cluster (e.g.,kubernetes-dashboard)


-->

## Alternatives
