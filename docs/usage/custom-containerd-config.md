---
title: Custom containerd Configuration
---

# Custom `containerd` Configuration
In case a `Shoot` cluster uses `containerd` (see [Kubernetes dockershim Removal](https://github.com/gardener/gardener/blob/master/docs/usage/docker-shim-removal.md)) for more information), it is possible to make the `containerd` process use custom configuration by modifying `/etc/containerd/config.toml`

One way of doing it is by using `sed`

## Setup
For the approach described below to work the config file should follow a specific format that `containerd config dump` applies. 
### Formatting
To format the config file the following should be executed
```bash
containerd config dump > /etc/containerd/config_formatted.toml && mv /etc/containerd/config_formatted.toml /etc/containerd/config.toml
```

## Working with the configuration
Example config file `cdexample`:
```toml
required_plugins = []
root = "/var/lib/containerd"
state = "/run/containerd"
version = 2

[cgroup]
  path = ""

[debug]
  address = ""
  format = ""

[plugins]

  [plugins."io.containerd.grpc.v1.cri"]
    device_ownership_from_security_context = false

    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = ""

       [plugins."io.containerd.grpc.v1.cri".registry.auths]

  [plugins."io.containerd.gc.v1.scheduler"]
    deletion_threshold = 0
    mutation_threshold = 100
```
### Explanation of placeholders: 
``` 
<<section_selector>> = \[<<section>>\] # selects only direct fields of <<section>>
					 = \[<<section>>   # selects direct fields and child sections/fields of <<section>>
					 = 1               # selects root fields

<<section/new_subsection>> = toml table
<<field>>                  = toml key = value pair
<<field_regex>>            = any BRE(basic regular expression) that matches a field
```
### Specifics
In `<<section_selector>>` \[ , \] and . should be escaped(see examples). For more information about special characters consult [this](https://www.regular-expressions.info/characters.html) and [this](https://www.gnu.org/software/sed/manual/html_node/BRE-vs-ERE.html). By default `sed` uses BRE(basic regular expressions). For more indepth understanding visit [sed's manual](https://www.gnu.org/software/sed/manual/sed.html)
### Printing a section
`sed -n /<<section_selector>>/,/^$/p /etc/containerd/config.toml`

``` bash
sed -n '1,/^$/p' cdexample
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
```

```bash 
sed -n '/\[plugins\."io\.containerd\.grpc\.v1\.cri"\]/,/^$/p' cdexample
# Result:
#   [plugins."io.containerd.grpc.v1.cri"]
#     device_ownership_from_security_context = false
#
```

```bash
sed -n '/\[plugins\."io\.containerd\.grpc\.v1\.cri"/,/^$/p' cdexample
# Result:
#   [plugins."io.containerd.grpc.v1.cri"]
#     device_ownership_from_security_context = false
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
```

### Deleting a section
`sed -i '/<<section_selector>>/,/^$/d' /etc/containerd/config.toml

```bash
sed -i '/\[plugins\."io\.containerd\.grpc\.v1\.cri"\]/,/^$/d' cdexample
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100
```

``` bash
sed -i '/\[plugins\."io.containerd\.grpc\.v1\.cri"/,/^$/d' cdexample 
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100

```

### Editing a section
#### Adding a field
`sed -i '/<<section>>/a<<field>>' /etc/containerd/config.toml`
Note: even if the added/edited fields are not indented properly this will not cause invalid toml. Only reaplying the formatting command above is nessesary when adding a new section and it's fields 

**Exception: If you want to add at a root field use `sed -i '1i<<field>>'

```bash
sed -i '1ifoo=bar' cdexample
# Result:
# foo=bar
# required_plugins = []
# root = "/var/lib/containerd"
....

```

```bash
sed -i '/\[plugins\."io\.containerd\.grpc\.v1\.cri"\]/afoo=bar' cdexample
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#   [plugins."io.containerd.grpc.v1.cri"]
# foo=bar
#     device_ownership_from_security_context = false
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100
```
#### Removing a field
`sed -i '/<<section>>/,/^$/s/.*<<field_regex>>.*//' /etc/containerd/config.toml`

```bash
sed -i '/\[plugins\."io\.containerd\.grpc\.v1\.cri"\]/,/^$/s/.*security.*//' cdexample 
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#   [plugins."io.containerd.grpc.v1.cri"]
#
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100
```
#### Editing a field 
`sed -i '/<<section>>/,/^$/s/.*<<field_regex>>.*/<<new_field>>/' /etc/containerd/config.toml`

```bash
sed -i '/\[plugins\."io\.containerd\.grpc\.v1\.cri"\]/,/^$/s/.*security.*/device_ownership_from_security_context = true/' cdexample 
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#   [plugins."io.containerd.grpc.v1.cri"]
# device_ownership_from_security_context = true
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100
```
### Adding a section

**NOTE:After adding all sections and fields run the formatting script above again**

#### Adding a top level section
`sed -i '$a<<section>>' /etc/containerd/config.toml`

```bash
sed -i '$a[proxy_plugins]' cdexample 
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#   [plugins."io.containerd.grpc.v1.cri"]
#     device_ownership_from_security_context = false
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100
# [proxy_plugins]
```
#### Adding a subsection
`sed -i '/<<section>>/a<<new_subsection>>' /etc/containerd/config.toml`

```bash
sed -i '/\[proxy_plugins\]/a[proxy_plugins."fuse-overlayfs"]' cdexample 
# Result:
# required_plugins = []
# root = "/var/lib/containerd"
# state = "/run/containerd"
# version = 2
#
# [cgroup]
#   path = ""
#
# [debug]
#   address = ""
#   format = ""
#
# [plugins]
#
#   [plugins."io.containerd.grpc.v1.cri"]
#     device_ownership_from_security_context = false
#
#     [plugins."io.containerd.grpc.v1.cri".registry]
#       config_path = ""
#
#        [plugins."io.containerd.grpc.v1.cri".registry.auths]
#
#   [plugins."io.containerd.gc.v1.scheduler"]
#     deletion_threshold = 0
#     mutation_threshold = 100
# [proxy_plugins]
# [proxy_plugins."fuse-overlayfs"]
```
#### Adding a fields
Use the same steps as described in Editing a section

## Validation
After applying the custom configuration run the formatting commands above and make sure the config file has the expected content
