# Gardener Node Agent

The goal of the `gardener-node-agent` is to bootstrap a machine into a worker node and maintain node-specific components, which run on the node and are unmanaged by Kubernetes (e.g. the `kubelet` service, systemd units, ...).

It effectively is a Kubernetes controller deployed onto the worker node.

## Architecture and Basic Design

![Design](./images/gardener-nodeagent-architecture.svg)

This figure visualizes the overall architecture of the `gardener-node-agent`. On the left side, it starts with an [`OperatingSystemConfig`](../extensions/operatingsystemconfig.md) resource (`OSC`) with a corresponding worker pool specific `cloud-config-<worker-pool>` secret being passed by reference through the userdata to a machine by the `machine-controller-manager` (MCM).

On the right side, the `cloud-config` secret will be extracted and used by the `gardener-node-agent` after being installed. Details on this can be found in the next section.

Finally, the `gardener-node-agent` runs a systemd service watching on secret resources located in the `kube-system` namespace like our `cloud-config` secret that contains the `OperatingSystemConfig`. When `gardener-node-agent` applies the OSC, it installs the `kubelet` + configuration on the worker node.

## Installation and Bootstrapping

This section describes how the `gardener-node-agent` is initially installed onto the worker node.

In the beginning, there is a very small bash script called [`gardener-node-init.sh`](../../pkg/component/extensions/operatingsystemconfig/original/components/containerd/templates/scripts/init.tpl.sh), which will be copied to `/var/lib/gardener-node-agent/gardener-node-init.sh` on the node with cloud-init data. This script's sole purpose is downloading and starting the `gardener-node-agent`. The binary artifact is extracted from an [OCI artifact](https://github.com/opencontainers/image-spec/blob/main/manifest.md) and lives at `/usr/local/bin/gardener-node-agent`. At the beginning, two architectures of the `gardener-node-agent` are supported: `amd64` and `x86`. The `kubelet` should also be contained in the same OCI artifact.

Along with the init script, a configuration for the `gardener-node-agent` is carried over to the worker node at `/var/lib/gardener-node-agent/configuration.yaml`. This configuration contains things like the shoot's `kube-apiserver` endpoint, the according certificates to communicate with it, the bootstrap token for the `kubelet`, and so on.

In a bootstrapping phase, the `gardener-node-agent` sets itself up as a systemd service. It also executes tasks that need to be executed before any other components are installed, e.g. formatting the data device for the `kubelet`.

## Reasoning

The `gardener-node-agent` is a replacement for what was called the `cloud-config-downloader` and the `cloud-config-executor`, both written in `bash`. The `gardener-node-agent` implements this functionality as a regular controller and feels more uniform in terms of maintenance.

With the new architecture we gain a lot, let's describe the most important gains here.

### Developer Productivity

Because we all develop in go day by day, writing business logic in `bash` is difficult, hard to maintain, almost impossible to test. Getting rid of almost all `bash` scripts which are currently in use for this very important part of the cluster creation process will enhance the speed of adding new features and removing bugs.

### Speed

Until now, the `cloud-config-downloader` runs in a loop every `60s` to check if something changed on the shoot which requires modifications on the worker node. This produces a lot of unneeded traffic on the API server and wastes time, it will sometimes take up to `60s` until a desired modification is started on the worker node.
By writing a "real" Kubernetes controller, we can watch for the `Node`, the `OSC` in the `Secret`, and the shoot-access token in the `secret`. If any of these object changed, and only then, the required action will take effect immediately.
This will speed up operations and will reduce the load on the API server of the shoot especially for large clusters.

## Scalability

The `cloud-config-downloader` adds a random wait time before restarting the `kubelet` in case the `kubelet` was updated or a configuration change was made to it. This is required to reduce the load on the API server and the traffic on the internet uplink. It also reduces the overall downtime of the services in the cluster because every `kubelet` restart transforms a node for several seconds into `NotReady` state which potentionally interrupts service availability.

Decision was made to keep the existing jitter mechanism which calculates the `kubelet-download-and-restart-delay-seconds` on the controller itself.

### Correctness

The configuration of the `cloud-config-downloader` is actually done by placing a file for every configuration item on the disk on the worker node. This was done because parsing the content of a single file and using this as a value in `bash` reduces to something like `VALUE=$(cat /the/path/to/the/file)`. Simple, but it lacks validation, type safety and whatnot.
With the `gardener-node-agent` we introduce a new API which is then stored in the `gardener-node-agent` `secret` and stored on disk in a single YAML file for comparison with the previous known state. This brings all benefits of type safe configuration.
Because actual and previous configuration are compared, removed files and units are also removed and stopped on the worker if removed from the `OSC`.

### Availability

Previously, the `cloud-config-downloader` simply restarted the systemd units on every change to the `OSC`, regardless which of the services changed. The `gardener-node-agent` first checks which systemd unit was changed, and will only restart these. This will prevent unneeded `kubelet` restarts.

### Future Development

The `gardener-node-agent` opens up the possibilty for further improvements.

Necessary restarts of the `kubelet` could be deterministic instead of the aforementioned random jittering. In that case, the `gardenlet` could add annotations across all nodes. As the `gardener-node-agent` watches the `Node` object, it could wait with `kubelet` restarts, OSC changes or react immediately. Critical changes could be performed in chunks of nodes in serial order, but an equal time spread is possible, too.
