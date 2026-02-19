## gardenadm join

Bootstrap control plane or worker nodes and join them to the cluster

### Synopsis

Bootstrap control plane or worker nodes and join them to the cluster.

This command helps to initialize and configure a node to join an existing self-hosted shoot cluster.
It ensures that the necessary configurations are applied and the node is properly registered as a control plane or worker node.

```
gardenadm join [flags]
```

### Examples

```
# Bootstrap a control plane node and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --control-plane <control-plane-address>

# Bootstrap a control plane node in a specific zone and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --control-plane --zone zone-a <control-plane-address>

# Bootstrap a worker node and join it to the cluster (by default, it is assigned to the first worker pool in the Shoot manifest)
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> <control-plane-address>

# Bootstrap a worker node in a specific worker pool and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --worker-pool-name <pool-name> <control-plane-address>

# Bootstrap a worker node in a specific zone and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --zone zone-b <control-plane-address>
```

### Options

```
      --bootstrap-token string       Bootstrap token for joining the cluster (create it with 'gardenadm token' on a control plane node)
      --ca-certificate bytesBase64   Base64-encoded certificate authority bundle of the control plane
      --control-plane                Create a new control plane instance on this node
  -h, --help                         help for join
  -w, --worker-pool-name string      Name of the worker pool to assign the joining node.
  -z, --zone string                  Zone in which this new node is being joined. Required when the worker pool has multiple zones configured, optional when a single zone is configured (automatically applied), and optional when no zones are configured.
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

