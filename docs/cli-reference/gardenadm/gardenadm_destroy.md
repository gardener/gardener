## gardenadm destroy

Destroy the infrastructure of a Self-Hosted Shoot Cluster

### Synopsis

Destroy the infrastructure of a Self-Hosted Shoot Cluster (machines, networks, etc.) that was created with
"gardenadm bootstrap".

Like "gardenadm bootstrap", this command deploys the provider extensions and machine-controller-manager to a
temporary bootstrap cluster. It restores the Infrastructure and Worker resources from the ShootState in the config
directory and deletes them, which deletes all machines and the infrastructure in the cloud provider account.

The config directory must contain the same manifests that were used for "gardenadm bootstrap" and the current
ShootState of the self-hosted shoot cluster.

Warning: the self-hosted shoot cluster must not be running anymore, or at least its machine-controller-manager must
be stopped. Otherwise, it recreates the machines that are deleted by this command.

```
gardenadm destroy [flags]
```

### Examples

```
# Destroy the infrastructure
gardenadm destroy --config-dir /path/to/manifests
```

### Options

```
  -d, --config-dir string   Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                help for destroy
  -k, --kubeconfig string   Path to the kubeconfig file pointing to the bootstrap cluster
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

