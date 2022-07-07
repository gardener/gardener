---
title: Custom containerd Configuration
---

# Custom `containerd` Configuration

In case a `Shoot` cluster uses `containerd` (see [this document](docker-shim-removal.md)) for more information), it is possible to make the `containerd` process load custom configuration files.
Gardener initializes `contaienerd` with the following statement:

```toml
imports = ["/etc/containerd/conf.d/*.toml"]
```

This means that all `*.toml` files in the `/etc/containerd/conf.d` directory will be imported and merged with the default configuration.
Please consult the [upstream `containerd` documentation](https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md#format) for more information.

> ⚠️ Note that this only applies to nodes which were newly created after `gardener/gardener@v1.51` was deployed. Existing nodes are not affected. 
