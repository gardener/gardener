## gardenadm discover

Conveniently download Gardener configuration resources from an existing garden cluster

### Synopsis

Conveniently download Gardener configuration resources from an existing garden cluster (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.)

```
gardenadm discover [flags]
```

### Examples

```
# Download the configuration
gardenadm discover <path-to-shoot-manifest>
```

### Options

```
  -d, --config-dir string        Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                     help for discover
  -k, --kubeconfig string        Path to the kubeconfig file pointing to the garden cluster
      --managed-infrastructure   Indicates whether Gardener will manage the shoot's infrastructure (network, domains, machines, etc.). Set this to true if using 'gardenadm bootstrap' for bootstrapping the shoot cluster. Set this to false if managing the infrastructure outside of Gardener. (default true)
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

