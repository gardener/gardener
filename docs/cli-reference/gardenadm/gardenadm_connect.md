## gardenadm connect

Deploy a gardenlet for further cluster management

### Synopsis

Deploy a gardenlet for further cluster management

```
gardenadm connect [flags]
```

### Examples

```
# Deploy a gardenlet
gardenadm connect
```

### Options

```
      --bootstrap-token string       Bootstrap token for connecting the autonomous shoot cluster to Gardener (create it with 'gardenadm token' in the garden cluster)
      --ca-certificate bytesBase64   Base64-encoded certificate authority bundle of the Gardener control plane
  -d, --config-dir string            Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                         help for connect
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

