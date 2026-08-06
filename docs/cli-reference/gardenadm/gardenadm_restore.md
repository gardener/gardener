## gardenadm restore

Restore a control plane node from an etcd backup and manifest files

### Synopsis

Restore a control plane node from an etcd backup and manifest files. Use this command to recover the
self-hosted shoot cluster's control plane node after a disaster (e.g., the control plane node is lost)
onto a new or existing node.

```
gardenadm restore [flags]
```

### Examples

```
# Restore a control plane node from an etcd backup
gardenadm restore --config-dir /path/to/manifests --backup-data-path /path/to/etcd-main/v2 --prior-node-name <name>
```

### Options

```
      --backup-data-path string   Local path on the node where the etcd backup data is stored. Expected structure: <backupBucketsRoot>/<bucketName>/<namespace>--<uid>/etcd-main/v2
  -d, --config-dir string         Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                      help for restore
      --prior-node-name string    The name of the prior control plane node. Required in order to cleanup stale resources.
  -z, --zone string               Availability zone of the new machine where the prior node is being restored to. Required if the control plane worker pool in the Shoot has multiple zones configured. Optional if exactly one zone is configured (applied automatically). Must not be set if no zones are configured.
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages self-hosted shoot clusters in the Gardener project.

