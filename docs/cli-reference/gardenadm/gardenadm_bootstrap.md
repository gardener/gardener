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

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

