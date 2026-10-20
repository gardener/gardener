## gardenadm destroy

Destroy the infrastructure of a Self-Hosted Shoot Cluster

### Synopsis

Destroy the infrastructure of a Self-Hosted Shoot Cluster (machines, networks, etc.) that was created with
"gardenadm bootstrap".

Like "gardenadm bootstrap", this command deploys the provider extensions and machine-controller-manager to a
temporary bootstrap cluster. It restores the Infrastructure and Worker resources from the ShootState in the config
directory and deletes them, which deletes all machines and the infrastructure in the cloud provider account.

The config directory must contain the same manifests that were used for "gardenadm bootstrap". The current ShootState
of the self-hosted shoot cluster is either computed from the shoot cluster (--shoot-kubeconfig) or read from the config
directory. The ShootState copied to the control plane machines by "gardenadm bootstrap" is outdated as soon as the
shoot cluster has changed its machines (e.g., created the machines of other worker pools).

Warning: the self-hosted shoot cluster must not be running anymore, or at least its machine-controller-manager must
be stopped. Otherwise, it recreates the machines that are deleted by this command.

Limitations:
- Only the machines and the infrastructure are destroyed. Other resources are left behind, e.g., the DNS record of
  the API server, the load balancer exposing the API server, and resources created by workloads in the cluster (load
  balancers of Services, volumes of PersistentVolumeClaims, etc.).
- Machines that are missing in the ShootState (e.g., because it is outdated) are not deleted.
- Re-running the command after machines or parts of the infrastructure have been deleted might create them again,
  because they are restored from the ShootState before they are deleted.

```
gardenadm destroy [flags]
```

### Examples

```
# Destroy the infrastructure, with the ShootState in the config directory
gardenadm destroy --config-dir /path/to/manifests

# Destroy the infrastructure, computing the current ShootState from the self-hosted shoot cluster
gardenadm destroy --config-dir /path/to/manifests --shoot-kubeconfig /path/to/shoot-kubeconfig
```

### Options

```
  -d, --config-dir string         Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                      help for destroy
  -k, --kubeconfig string         Path to the kubeconfig file pointing to the bootstrap cluster
      --shoot-kubeconfig string   Path to the kubeconfig file pointing to the self-hosted shoot cluster. If set, the current ShootState is computed from the shoot cluster instead of being read from the config directory
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

