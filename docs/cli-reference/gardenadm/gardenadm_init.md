## gardenadm init

Bootstrap the first control plane node

### Synopsis

Bootstrap the first control plane node

```
gardenadm init [flags]
```

### Examples

```
# Bootstrap the first control plane node
gardenadm init --config-dir /path/to/manifests

# Bootstrap the first control plane node in a specific zone (required when multiple zones are configured in the `Shoot` resource)
gardenadm init --config-dir /path/to/manifests --zone zone-a
```

### Options

```
  -d, --config-dir string    Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                 help for init
      --use-bootstrap-etcd   If set, the control plane continues using the bootstrap etcd instead of transitioning to etcd-druid. This is useful for testing purposes to save time.
  -z, --zone string          Availability zone for the new node. Required if the control plane worker pool in the Shoot has multiple zones configured. Optional if exactly one zone is configured (applied automatically). Must not be set if no zones are configured.
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

