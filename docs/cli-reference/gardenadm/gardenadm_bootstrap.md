## gardenadm bootstrap

Bootstrap the infrastructure for an Autonomous Shoot Cluster

### Synopsis

Bootstrap the infrastructure for an Autonomous Shoot Cluster (networks, machines, etc.)

```
gardenadm bootstrap [flags]
```

### Examples

```
# Bootstrap the infrastructure
gardenadm bootstrap --config-dir /path/to/manifests
```

### Options

```
  -d, --config-dir string   Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                help for bootstrap
  -k, --kubeconfig string   Path to the kubeconfig file pointing to the KinD cluster
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

