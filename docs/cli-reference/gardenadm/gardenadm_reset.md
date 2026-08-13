## gardenadm reset

Reset control plane or worker nodes and remove them from the cluster

### Synopsis

Reset control plane or worker nodes and remove them from the cluster.

This command helps to remove a node from an existing self-hosted shoot cluster.
It ensures that the components deployed to this node are removed and the node is properly deregistered as a control plane or worker node.

```
gardenadm reset [flags]
```

### Examples

```
# Reset a node and remove it from the cluster
gardenadm reset --token <token> --ca-certificate <ca-cert> <control-plane-address>
```

### Options

```
      --ca-certificate bytesBase64   Base64-encoded certificate authority bundle of the control plane
      --drain-timeout duration       Timeout for draining the node (default 2h0m0s)
  -h, --help                         help for reset
      --token string                 Token for removing the node from the cluster (create it with 'gardenadm token' on a control plane node)
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

