---
title: Custom containerd Configuration
weight: 1
---

# Custom `containerd` Configuration

In case a `Shoot` cluster uses `containerd`, it is possible to make the `containerd` process load custom configuration files.
Gardener initializes `containerd` with the following statement:

```toml
imports = ["/etc/containerd/conf.d/*.toml"]
```

This means that all `*.toml` files in the `/etc/containerd/conf.d` directory will be imported and merged with the default configuration.
To prevent unintended configuration overwrites, please be aware that containerd merges config sections, not individual keys (see [here](https://github.com/containerd/containerd/issues/5837#issuecomment-894840240) and [here](https://github.com/gardener/gardener/pull/7316)).
Please consult the [upstream `containerd` documentation](https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md#format) for more information.

> ⚠️ Note that this only applies to nodes which were newly created after `gardener/gardener@v1.51` was deployed. Existing nodes are not affected. 
