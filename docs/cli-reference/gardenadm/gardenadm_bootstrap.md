## gardenadm bootstrap

Bootstrap the infrastructure for a Self-Hosted Shoot Cluster

### Synopsis

Bootstrap the infrastructure for a Self-Hosted Shoot Cluster (networks, machines, etc.)

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
      --bastion-ingress-cidr strings   Restrict bastion host ingress to the given CIDRs. Defaults to your system's public IPs (IPv4 and/or IPv6) as detected using https://ipify.org/.
  -d, --config-dir string              Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                           help for bootstrap
  -k, --kubeconfig string              Path to the kubeconfig file pointing to the bootstrap cluster
      --kubeconfig-output string       Path where the kubeconfig file for the shoot cluster should be written to. If not set, the kubeconfig is not written to disk. Set to '-' to write to stdout.
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

