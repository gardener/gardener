## gardenadm discover existing

Download Gardener configuration resources for an existing Shoot in the garden cluster

### Synopsis

Download Gardener configuration resources (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.) from an existing garden cluster for an existing Shoot.

```
gardenadm discover existing [flags]
```

### Examples

```
# Download the configuration for an existing Shoot
gardenadm discover existing --name <name> --namespace <namespace>
```

### Options

```
  -d, --config-dir string        Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                     help for existing
  -k, --kubeconfig string        Path to the kubeconfig file pointing to the garden cluster
      --managed-infrastructure   Indicates whether Gardener will manage the shoot's infrastructure (network, domains, machines, etc.). Set this to true if using 'gardenadm bootstrap' for bootstrapping the shoot cluster. Set this to false if managing the infrastructure outside of Gardener. (default true)
      --name string              Name of an existing Shoot in the garden cluster to discover resources for.
      --namespace string         Namespace of an existing Shoot in the garden cluster to discover resources for.
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm discover](gardenadm_discover.md)	 - Conveniently download Gardener configuration resources from an existing garden cluster

