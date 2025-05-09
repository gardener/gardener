## gardenadm init

Bootstrap the first control plane node

### Synopsis

Bootstrap the first control plane node

```
gardenadm init [flags]
```

### Examples

```
# Bootstrap the first control plane node
gardenadm init --config-dir /path/to/manifests
```

### Options

```
  -d, --config-dir string   Path to a directory containing the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.
  -h, --help                help for init
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm](gardenadm.md)	 - gardenadm bootstraps and manages autonomous shoot clusters in the Gardener project.

