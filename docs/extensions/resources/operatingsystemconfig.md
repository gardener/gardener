---
title: OperatingSystemConfig
---
# Contract: `OperatingSystemConfig` Resource

Gardener uses the machine API and leverages the functionalities of the [machine-controller-manager](https://github.com/gardener/machine-controller-manager) (MCM) in order to manage the worker nodes of a shoot cluster.
The machine-controller-manager itself simply takes a reference to an OS-image and (optionally) some user-data (a script or configuration that is executed when a VM is bootstrapped), and forwards both to the provider's API when creating VMs.
MCM does not have any restrictions regarding supported operating systems as it does not modify or influence the machine's configuration in any way - it just creates/deletes machines with the provided metadata.

Consequently, Gardener needs to provide this information when interacting with the machine-controller-manager.
This means that basically every operating system is possible to be used, as long as there is some implementation that generates the OS-specific configuration in order to provision/bootstrap the machines.

:warning: Currently, there are a few requirements of pre-installed components that must be present in all OS images:

1. [containerd](https://containerd.io/)
   1. [ctr (client CLI)](https://github.com/projectatomic/containerd/blob/master/docs/cli.md/)
   1. `containerd` must listen on its default socket path: `unix:///run/containerd/containerd.sock`
   1. `containerd` must be configured to work with the default configuration file in: `/etc/containerd/config.toml` (eventually created by Gardener).
1. [systemd](https://www.freedesktop.org/wiki/Software/systemd/)

The reasons for that will become evident later.

## What does the user-data bootstrapping the machines contain?

Gardener installs a few components onto every worker machine in order to allow it to join the shoot cluster.
There is the `kubelet` process, some scripts for continuously checking the health of `kubelet` and `containerd`, but also configuration for log rotation, CA certificates, etc.
You can find the complete configuration [at the components folder](../../../pkg/component/extensions/operatingsystemconfig/original/components). We are calling this the "original" user-data.

## How does Gardener bootstrap the machines?

`gardenlet` makes use of `gardener-node-agent` to perform the bootstrapping and reconciliation of systemd units and files on the machine.
Please refer to [this document](../../concepts/node-agent.md#installation-and-bootstrapping) for a first overview.

Usually, you would submit all the components you want to install onto the machine as part of the user-data during creation time.
However, some providers do have a size limitation (around ~16KB) for that user-data.
That's why we do not send the "original" user-data to the machine-controller-manager (who then forwards it to the provider's API).
Instead, we only send a small "init" script that bootstrap the [`gardener-node-agent`](../../concepts/node-agent.md).
It fetches the "original" content from a `Secret` and applies it on the machine directly.
This way we can extend the "original" user-data without any size restrictions (except for the `1 MB` limit for `Secret`s).

The high-level flow is as follows:

1. For every worker pool `X` in the `Shoot` specification, Gardener creates a `Secret` named `cloud-config-<X>` in the `kube-system` namespace of the shoot cluster. The secret contains the "original" `OperatingSystemConfig` (i.e., systemd units and files for `kubelet`).
1. Gardener generates a kubeconfig with minimal permissions just allowing reading these secrets. It is used by the `gardener-node-agent` later.
1. Gardener provides the `gardener-node-init.sh` bash script and the machine image stated in the `Shoot` specification to the machine-controller-manager.
1. Based on this information, the machine-controller-manager creates the VM.
1. After the VM has been provisioned, the `gardener-node-init.sh` script starts, fetches the `gardener-node-agent` binary, and starts it.
1. The `gardener-node-agent` will read the `gardener-node-agent-<X>` `Secret` for its worker pool (containing the "original" `OperatingSystemConfig`), and reconciles it.

The `gardener-node-agent` can update itself in case of newer Gardener versions, and it performs a continuous reconciliation of the systemd units and files in the provided `OperatingSystemConfig` (just like any other Kubernetes controller).

## What needs to be implemented to support a new operating system?

As part of the [`Shoot` reconciliation flow](../../concepts/gardenlet.md#shoot-controller), `gardenlet` will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: OperatingSystemConfig
metadata:
  name: pool-01-original
  namespace: default
spec:
  type: <my-operating-system>
  purpose: reconcile
  units:
  - name: containerd.service
    dropIns:
    - name: 10-containerd-opts.conf
      content: |
        [Service]
        Environment="SOME_OPTS=--foo=bar"
  - name: containerd-monitor.service
    command: start
    enable: true
    content: |
      [Unit]
      Description=Containerd-monitor daemon
      After=kubelet.service
      [Install]
      WantedBy=multi-user.target
      [Service]
      Restart=always
      EnvironmentFile=/etc/environment
      ExecStart=/opt/bin/health-monitor containerd
  files:
  - path: /var/lib/kubelet/ca.crt
    permissions: 0644
    encoding: b64
    content:
      secretRef:
        name: default-token-5dtjz
        dataKey: token
  - path: /etc/sysctl.d/99-k8s-general.conf
    permissions: 0644
    content:
      inline:
        data: |
          # A higher vm.max_map_count is great for elasticsearch, mongo, or other mmap users
          # See https://github.com/kubernetes/kops/issues/1340
          vm.max_map_count = 135217728
```

In order to support a new operating system, you need to write a controller that watches all `OperatingSystemConfig`s with `.spec.type=<my-operating-system>`.
For those it shall generate a configuration blob that fits to your operating system.

`OperatingSystemConfig`s can have two purposes: either `provision` or `reconcile`.

### `provision` Purpose

The `provision` purpose is used by `gardenlet` for the user-data that it later passes to the machine-controller-manager (and then to the provider's API) when creating new VMs.
It contains the `gardener-node-init.sh` script and systemd unit.

The OS controller has to translate the `.spec.units` and `.spec.files` into configuration that fits to the operating system.
For example, a Flatcar controller might generate a [CoreOS cloud-config](https://github.com/flatcar/coreos-cloudinit/blob/flatcar-master/Documentation/cloud-config-examples.md) or [Ignition](https://coreos.com/ignition/docs/latest/what-is-ignition.html), SLES might generate [cloud-init](https://cloudinit.readthedocs.io/en/latest/), and others might simply generate a bash script translating the `.spec.units` into `systemd` units, and `.spec.files` into real files on the disk.

> ⚠️ Please avoid mixing in additional systemd units or files - this step should just translate what `gardenlet` put into `.spec.units` and `.spec.files`.

After generation, extension controllers are asked to store their OS config inside a `Secret` (as it might contain confidential data) in the same namespace.
The secret's `.data` could look like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: osc-result-pool-01-original
  namespace: default
  ownerReferences:
  - apiVersion: extensions.gardener.cloud/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: OperatingSystemConfig
    name: pool-01-original
    uid: 99c0c5ca-19b9-11e9-9ebd-d67077b40f82
data:
  cloud_config: base64(generated-user-data)
```

Finally, the secret's metadata must be provided in the `OperatingSystemConfig`'s `.status` field:

```yaml
...
status:
  cloudConfig:
    secretRef:
      name: osc-result-pool-01-original
      namespace: default
  lastOperation:
    description: Successfully generated cloud config
    lastUpdateTime: "2019-01-23T07:45:23Z"
    progress: 100
    state: Succeeded
    type: Reconcile
  observedGeneration: 5
```

### `reconcile` Purpose

The `reconcile` purpose contains the "original" `OperatingSystemConfig` (which is later stored in `Secret`s in the shoot's `kube-system` namespace (see step 1)). This is downloaded and applies late (see step 5).

The OS controller does not need to translate anything here, but it has the option to provide additional systemd units or files via the `.status` field:

```yaml
status:
  extensionUnits:
  - name: my-custom-service.service
    command: start
    enable: true
    content: |
      [Unit]
      // some systemd unit content
  extensionFiles:
  - path: /etc/some/file
    permissions: 0644
    content:
      inline:
        data: some-file-content
  lastOperation:
    description: Successfully generated cloud config
    lastUpdateTime: "2019-01-23T07:45:23Z"
    progress: 100
    state: Succeeded
    type: Reconcile
  observedGeneration: 5
```

The `gardener-node-agent` will merge `.spec.units` and `.status.extensionUnits` as well as `.spec.files` and `.status.extensionFiles` when applying.

You can find an example implementation [here](../../../pkg/provider-local/controller/operatingsystemconfig/actuator.go).

As described above, the "original" user-data must be re-applicable to allow in-place updates.
The way how this is done is specific to the generated operating system config (e.g., for CoreOS cloud-init the command is `/usr/bin/coreos-cloudinit --from-file=<path>`, whereas SLES would run `cloud-init --file <path> single -n write_files --frequency=once`).
Consequently, besides the generated OS config, the extension controller must also provide a command for re-application an updated version of the user-data.
As visible in the mentioned examples, the command requires a path to the user-data file.
As soon as Gardener detects that the user data has changed it will reload the systemd daemon and restart all the units provided in the `.status.units[]` list (see the below example). The same logic applies during the very first application of the whole configuration.

After generation, extension controllers are asked to store their OS config inside a `Secret` (as it might contain confidential data) in the same namespace.
The secret's `.data` could look like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: osc-result-pool-01-original
  namespace: default
  ownerReferences:
  - apiVersion: extensions.gardener.cloud/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: OperatingSystemConfig
    name: pool-01-original
    uid: 99c0c5ca-19b9-11e9-9ebd-d67077b40f82
data:
  cloud_config: base64(generated-user-data)
```

Finally, the secret's metadata, the OS-specific command to re-apply the configuration, and the list of `systemd` units that shall be considered to be restarted if an updated version of the user-data is re-applied must be provided in the `OperatingSystemConfig`'s `.status` field:

```yaml
...
status:
  cloudConfig:
    secretRef:
      name: osc-result-pool-01-original
      namespace: default
  lastOperation:
    description: Successfully generated cloud config
    lastUpdateTime: "2019-01-23T07:45:23Z"
    progress: 100
    state: Succeeded
    type: Reconcile
  observedGeneration: 5
  units:
  - docker-monitor.service
```

Once the `.status` indicates that the extension controller finished reconciling Gardener will continue with the next step of the shoot reconciliation flow.

### Bootstrap Tokens

`gardenlet` adds a file with the content `<<BOOTSTRAP_TOKEN>>` to the `OperatingSystemConfig` with purpose `provision` and sets `transmitUnencoded=true`.
This instructs the responsible OS extension to pass this file (with its content in clear-text) to the corresponding `Worker` resource.

`machine-controller-manager` makes sure that:

- a bootstrap token gets created per machine
- the `<<BOOTSTRAP_TOKEN>>` string in the user data of the machine gets replaced by the generated token

After the machine has been bootstrapped, the token secret in the shoot cluster gets deleted again.

The token is used to bootstrap [Gardener Node Agent](../../concepts/node-agent.md) and `kubelet`.

## CRI Support

Gardener supports specifying a Container Runtime Interface (CRI) configuration in the `OperatingSystemConfig` resource. If the `.spec.cri` section exists, then the `name` property is mandatory. The only supported value for `cri.name` at the moment is: `containerd`.
For example:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: OperatingSystemConfig
metadata:
  name: pool-01-original
  namespace: default
spec:
  type: <my-operating-system>
  purpose: reconcile
  cri:
    name: containerd
#   cgroupDriver: cgroupfs # or systemd
    containerd:
      sandboxImage: registry.k8s.io/pause
#     registries:
#     - upstream: docker.io
#       server: https://registry-1.docker.io
#       hosts:
#       - url: http://<service-ip>:<port>]
#     plugins:
#     - op: add # add (default) or remove
#       path: [io.containerd.grpc.v1.cri, containerd]
#       values: '{"default_runtime_name": "runc"}'
...
```

To support `containerd`, an OS extension must satisfy the following criteria:

1. The operating system must have built-in [containerd](https://containerd.io/) and [ctr (client CLI)](https://github.com/projectatomic/containerd/blob/master/docs/cli.md/).
1. `containerd` must listen on its default socket path: `unix:///run/containerd/containerd.sock`
1. `containerd` must be configured to work with the default configuration file in: `/etc/containerd/config.toml` (Created by Gardener).

For a convenient handling, [gardener-node-agent](../../concepts/node-agent.md) can manage various aspects of containerd's config, e.g. the registry configuration, if given in the `OperatingSystemConfig`.
Any Gardener extension which needs to modify the config, should check the functionality exposed through this API first.
If applicable, adjustments can be implemented through mutating webhooks, acting on the created or updated `OperatingSystemConfig` resource.

If CRI configurations are not supported, it is recommended to create a validating webhook running in the garden cluster that prevents specifying the `.spec.providers.workers[].cri` section in the `Shoot` objects.

### cgroup driver

For Shoot clusters using Kubernetes < 1.31, Gardener is setting the kubelet's cgroup driver to [`cgroupfs`](https://kubernetes.io/docs/setup/production-environment/container-runtimes/#cgroupfs-cgroup-driver) and containerd's cgroup driver is unmanaged. For Shoot clusters using Kubernetes 1.31+, Gardener is setting both kubelet's and containerd's cgroup driver to [`systemd`](https://kubernetes.io/docs/setup/production-environment/container-runtimes/#systemd-cgroup-driver).

The `systemd` cgroup driver is a requirement for operating systems using [cgroup v2](https://kubernetes.io/docs/concepts/architecture/cgroups/). It's important to ensure that both kubelet and the container runtime (containerd) are using the same cgroup driver to avoid potential issues.

OS extensions might also overwrite the cgroup driver for containerd and kubelet.

## References and Additional Resources

- [`OperatingSystemConfig` API (Golang Specification)](../../../pkg/apis/extensions/v1alpha1/types_operatingsystemconfig.go)
- [Gardener Node Agent](../../concepts/node-agent.md)
