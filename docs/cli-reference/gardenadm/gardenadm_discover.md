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
gardenadm discover --kubeconfig ~/.kube/config
```

### Options

```
  -h, --help                help for discover
  -k, --kubeconfig string   Path to the kubeconfig file pointing to the garden cluster
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

