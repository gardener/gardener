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
gardenadm bootstrap --kubeconfig ~/.kube/config
```

### Options

```
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

