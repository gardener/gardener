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

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

