# Gardener Node Agent

The goal of the `gardener-node-agent` is to bootstrap a machine into a worker node and maintain node-specific components, which run on the node and are unmanaged by Kubernetes (e.g. the controller runtime, the kubelet service, ...).

It effectively is a Kubernetes controller deployed onto the worker node.

## Basic Design

In this section it is described how the `gardener-node-agent` works, what its responsibilities are and how it is installed onto the worker node.

To install the `gardener-node-agent` onto a worker node, there is a very small bash script called `gardener-node-init.sh`, which is installed on the node with cloud-init data. This script's sole purpose is downloading and starting the `gardener-node-agent`. The binary artifact is downloaded as an [OCI artifact](https://github.com/opencontainers/image-spec/blob/main/manifest.md), removing the `docker` dependency on a worker node. At the beginning, two architectures of the `gardener-node-agent` are supported: `amd64` and `x86`. In the same manner, the kubelet has to be provided as an OCI artifact.

Along with the init script, a configuration for the `gardener-node-agent` is carried onto the worker node at `/etc/gardener/node-agent.config`. This configuration contains things like the shoot's kube-apiserver endpoint, the according certificates to communicate with it, the bootstrap token for the kubelet, and so on.

In a bootstrapping phase, the `gardener-node-agent` sets itself up as a systemd service. It also executes tasks that need to be executed before any other components are installed, e.g. formatting the data device for the kubelet.

After the bootstrap phase, the `gardener-node-agent` runs a systemd service watching on secret resources located in the `kube-system` namespace. There is a secret resource that contains the `OperatingSystemConfig` to reconcile. The OSC secret exists for every worker group of the shoot cluster and is named accordingly. Applying the OSC finally installs the kubelet + configuration on the worker node.

## Architecture

![Design](./images/gardener-nodeagent-architecture.drawio.svg)

This figure visualizes the overall architecture of the `gardener-node-agent`. It starts with the downloader OSC being transferred through the userdata to a machine through the machine-controller-manager (MCM). The bootstrap phase of the `gardener-node-agent` will then happen as described in the previous section.

## Reasoning

The `gardener-node-agent` is a replacement for what was called the `cloud-config-downloader` and the `cloud-config-executor`, both written in `bash`. The `gardener-node-agent` gets rid of the sheer complexity of these two scripts, combined with scalability and performance issues urges their removal.

With the new Architecture we gain a lot, let's describe the most important gains here.

### Developer Productivity

Because we all develop in go day by day, writing business logic in `bash` is difficult, hard to maintain, almost impossible to test. Getting rid of almost all `bash` scripts which are currently in use for this very important part of the cluster creation process will enhance the speed of adding new features and removing bugs.

### Speed

Until now, the `cloud-config-downloader` runs in a loop every 60sec to check if something changed on the shoot which requires modifications on the worker node. This produces a lot of unneeded traffic on the api-server and wastes time, it will sometimes take up to 60sec until a desired modification is started on the worker node.
By using the controller-runtime we can watch for the `node`, the`OSC` in the `secret`, and the shoot-access-token in the `secret`. If any of these object changed, and only then, the required action will take effect immediately.
This will speed up operations and will reduce the load on the api-server of the shoot dramatically.

## Scalability

Actually the `cloud-config-downloader` add a random wait time before restarting the `kubelet` in case the `kubelet` was updated or a configuration change was made to it. This is required to reduce the load on the API server and the traffic on the internet uplink. It also reduces the overall downtime of the services in the cluster because every `kubelet` restart takes a node for several seconds into `NotReady` state which eventually interrupts service availability.

```
TODO: The `gardener-node-agent` could do this in a much intelligent way because it watches the `node` object. The gardenlet could add some annotation which tells the `gardener-node-agent` to wait for the kubelet in a coordinated manner. The coordination could be in chunks of nodes and wait for them to finish and then start with the next chunk. Also a equal time spread is possible.
```

Decision was made to keep the existing jitter mechanism which calculates the kubelet-download-and-restart-delay-seconds on the controller itself.

### Correctness

The configuration of the `cloud-config-downloader` is actually done by placing a file for every configuration item on the disk on the worker node. This was done because parsing the content of a single file and using this as a value in `bash` reduces to something like `VALUE=$(cat /the/path/to/the/file)`. Simple but lacks validation, type safety and whatnot.
With the `gardener-node-agent` we introduce a new API which is then stored in the `gardener-node-agent` `secret` and stored on disc in a single yaml file for comparison with the previous known state. This brings all benefits of type safe configuration.
Because actual and previous configuration are compared, removed files and units are also removed and stopped on the worker if removed from the `OSC`.

### Availability

Previously the `cloud-config-downloader` simply restarted the `systemd-units` on every change to the `OSC`, regardless which of the services changed. The `gardener-node-agent` first checks which systemd-unit was changed, and will only restart these. This will remove unneeded `kubelet` restarts.
