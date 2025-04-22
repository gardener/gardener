## gardenadm join

Bootstrap worker nodes and join them to the cluster

### Synopsis

Bootstrap worker nodes and join them to the cluster.

This command helps to initialize and configure a node to join an existing autonomous shoot cluster.
It ensures that the necessary configurations are applied and the node is properly registered as a worker or control plane node.

Note that further control plane nodes cannot be joined currently.

```
gardenadm join [flags]
```

### Examples

```
# Bootstrap a worker node and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --gardener-node-agent-secret-name <secret-name> <control-plane-address>
```

### Options

```
      --bootstrap-token string                   Bootstrap token for joining the cluster (create it with gardenadm token)
      --ca-certificate bytesBase64               Base64-encoded certificate authority bundle of the control plane
      --gardener-node-agent-secret-name string   Name of the Secret from which gardener-node-agent should download its operating system configuration
  -h, --help                                     help for join
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

