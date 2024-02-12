---
title: containerd Registry Configuration
---

# `containerd` Registry Configuration

containerd supports configuring registries and mirrors. Using this native containerd feature, Shoot owners can configure containerd to use public or private mirrors for a given upstream registry. More details about the registry configuration can be found in the [corresponding upstream documentation](https://github.com/containerd/containerd/blob/main/docs/hosts.md).

### `containerd` Registry Configuration Patterns

At the time of writing this document, containerd support two patterns for configuring registries/mirrors.

> Note: Trying to use both of the patterns at the same time is not supported by containerd. Only one of the configuration patterns has to be followed strictly.

##### Old and Deprecated Pattern

The old and deprecated pattern is specifying `registry.mirrors` and `registry.configs` in the containerd's config.toml file. See the [upstream documentation](https://github.com/containerd/containerd/blob/main/docs/cri/registry.md).
Example of the old and deprecated pattern:

```toml
version = 2

[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
      endpoint = ["https://public-mirror.example.com"]
```

In the above example, containerd is configured to first try to pull `docker.io` images from a configured endpoint (`https://public-mirror.example.com`). If the image is not available in `https://public-mirror.example.com`, then containerd will fall back to the upstream registry (`docker.io`) and will pull the image from there.

##### Hosts Directory Pattern

The hosts directory pattern is the new and recommended pattern for configuring registries. It is available starting `containerd@v1.5.0`. See the [upstream documentation](https://github.com/containerd/containerd/blob/main/docs/hosts.md).
The above example in the hosts directory pattern looks as follows.
The `/etc/containerd/config.toml` file has the following section:

```toml
version = 2

[plugins."io.containerd.grpc.v1.cri".registry]
   config_path = "/etc/containerd/certs.d"
```

The following hosts directory structure has to be created:
```
$ tree /etc/containerd/certs.d
/etc/containerd/certs.d
└── docker.io
    └── hosts.toml
```

Finally, for the `docker.io` upstream registry, we configure a `hosts.toml` file as follows:

```toml
server = "https://registry-1.docker.io"

[host."http://public-mirror.example.com"]
  capabilities = ["pull", "resolve"]
```

### Configuring `containerd` Registries for a Shoot

Gardener supports configuring `containerd` registries on a Shoot using the new [hosts directory pattern](https://github.com/containerd/containerd/blob/main/docs/hosts.md). For each Shoot Node, Gardener creates the `/etc/containerd/certs.d` directory and adds the following section to the containerd's `/etc/containerd/config.toml` file:

```toml
[plugins."io.containerd.grpc.v1.cri".registry] # gardener-managed
   config_path = "/etc/containerd/certs.d"
```

This allows Shoot owners to use the [hosts directory pattern](https://github.com/containerd/containerd/blob/main/docs/hosts.md) to configure registries for containerd. To do this, the Shoot owners need to create a directory under `/etc/containerd/certs.d` that is named with the upstream registry host name. In the newly created directory, a `hosts.toml` file needs to be created. For more details, see the [hosts directory pattern section](#hosts-directory-pattern) and the [upstream documentation](https://github.com/containerd/containerd/blob/main/docs/hosts.md).

### The registry-cache Extension

There is a Gardener-native extension named [registry-cache](https://github.com/gardener/gardener-extension-registry-cache) that supports:
- Configuring containerd registry mirrors based on the above-described contract. The feature is added in [registry-cache@v0.6.0](https://github.com/gardener/gardener-extension-registry-cache/releases/tag/v0.6.0).
- Running pull through cache(s) in the Shoot.

For more details, see the [registry-cache documentation](https://github.com/gardener/gardener-extension-registry-cache/blob/main/README.md).
