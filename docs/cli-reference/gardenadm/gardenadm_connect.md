## gardenadm connect

Deploy a gardenlet for further cluster management

### Synopsis

Deploy a gardenlet for further cluster management.

The command can be executed on a control plane machine of the self-hosted shoot cluster or anywhere else. The
self-hosted shoot cluster is targeted via the KUBECONFIG environment variable (defaults to /etc/kubernetes/admin.conf).
Outside of a control plane machine, the --config-dir flag must point to the resources used for creating the cluster.

```
gardenadm connect [flags]
```

### Examples

```
# Deploy a gardenlet (on a control plane machine)
gardenadm connect --bootstrap-token <token> --ca-certificate <ca> https://api.garden.example.com

# Deploy a gardenlet (from outside the self-hosted shoot cluster)
KUBECONFIG=/path/to/self-hosted-shoot/kubeconfig gardenadm connect --config-dir /path/to/manifests --bootstrap-token <token> --ca-certificate <ca> https://api.garden.example.com
```

### Options

```
      --bootstrap-token string       Bootstrap token for connecting the self-hosted shoot cluster to a garden cluster (create it with 'gardenadm token' in the garden cluster)
      --ca-certificate bytesBase64   Base64-encoded certificate authority bundle of the Gardener control plane
  -d, --config-dir string            Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
      --force                        Forces the deployment of gardenlet, even if it already exists
  -h, --help                         help for connect
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

