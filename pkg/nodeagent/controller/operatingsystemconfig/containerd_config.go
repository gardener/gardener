// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/structuredmap"
)

type (
	containerdConfigFileVersion int
	containerdConfigPathName    int

	containerdConfigPathMap         map[containerdConfigPathName]structuredmap.Path
	containerdConfigPathMapVersions map[containerdConfigFileVersion]containerdConfigPathMap

	replacementMap map[*structuredmap.Path]structuredmap.Path
)

const (
	registryConfigPath containerdConfigPathName = iota
	importsPath
	sandboxImagePath
	cgroupDriverPath
	cniPluginPath
)

var (
	// containerdConfigPaths is a nested map that contains the paths/keys for certain configuration options that change across different config file versions.
	containerdConfigPaths = containerdConfigPathMapVersions{
		1: {
			registryConfigPath: {"plugins", "io.containerd.grpc.v1.cri", "registry", "config_path"},
			sandboxImagePath:   {"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"},
			cgroupDriverPath:   {"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"},
			cniPluginPath:      {"plugins", "io.containerd.grpc.v1.cri", "cni", "bin_dir"},
		},
		2: {
			registryConfigPath: {"plugins", "io.containerd.grpc.v1.cri", "registry", "config_path"},
			sandboxImagePath:   {"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"},
			cgroupDriverPath:   {"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"},
			cniPluginPath:      {"plugins", "io.containerd.grpc.v1.cri", "cni", "bin_dir"},
		},
		3: {
			registryConfigPath: {"plugins", "io.containerd.cri.v1.images", "registry", "config_path"},
			sandboxImagePath:   {"plugins", "io.containerd.cri.v1.runtime", "sandbox_image"},
			cgroupDriverPath:   {"plugins", "io.containerd.cri.v1.runtime", "containerd", "runtimes", "runc", "options", "SystemdCgroup"},
			cniPluginPath:      {"plugins", "io.containerd.cri.v1.runtime", "cni", "bin_dir"},
		},
	}

	// pluginPathReplacements is a map that contains replacements for paths brought in through an osc plugin config
	pluginPathReplacements = replacementMap{
		{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes"}: {"plugins", "io.containerd.cri.v1.runtime", "containerd", "runtimes"},
	}
)

// getContainerdConfigFileVersion obtains the containerd configuration file version from the configuration file.
func getContainerdConfigFileVersion(config map[string]any) (containerdConfigFileVersion, error) {
	version, ok := config["version"]
	if !ok {
		// Config file versions 2 and 3 must contain a version header.
		// If it cannot be found, it therefore must be version 1.
		return 1, nil
	}

	i, ok := version.(int64)
	if !ok {
		return 0, fmt.Errorf("cannot assert containerd config file version \"%v\" as an int64", version)
	}

	if i < 1 || i > 3 {
		return 0, fmt.Errorf("unsupported containerd config file version %d", i)
	}

	return containerdConfigFileVersion(i), nil
}

// ensureContainerdDefaultConfig invokes the 'containerd' and saves the resulting default configuration.
func (r *Reconciler) ensureContainerdDefaultConfig(ctx context.Context) error {
	exists, err := r.FS.Exists(configFile)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	output, err := Exec(ctx, "containerd", "config", "default")
	if err != nil {
		return err
	}

	return r.FS.WriteFile(configFile, output, 0644)
}

// ensureContainerdConfiguration sets the configuration for containerd.
func (r *Reconciler) ensureContainerdConfiguration(log logr.Logger, criConfig *extensionsv1alpha1.CRIConfig) error {
	config, err := r.FS.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("unable to read containerd config.toml: %w", err)
	}

	content := map[string]any{}

	if err = toml.Unmarshal(config, &content); err != nil {
		return fmt.Errorf("unable to decode containerd default config: %w", err)
	}

	configFileVersion, err := getContainerdConfigFileVersion(content)
	if err != nil {
		return err
	}

	type patch struct {
		name  string
		path  structuredmap.Path
		setFn structuredmap.SetFn
	}

	patches := []patch{
		{
			name: "registry config path",
			path: containerdConfigPaths[configFileVersion][registryConfigPath],
			setFn: func(_ any) (any, error) {
				return certsDir, nil
			},
		},
		{
			name: "imports paths",
			path: structuredmap.Path{"imports"},
			setFn: func(value any) (any, error) {
				importPath := path.Join(configDir, "*.toml")

				imports, ok := value.([]any)
				if !ok {
					return []string{importPath}, nil
				}

				for _, imp := range imports {
					path, ok := imp.(string)
					if !ok {
						continue
					}

					if path == importPath {
						return value, nil
					}
				}

				return append(imports, importPath), nil
			},
		},
		{
			name: "sandbox image",
			path: containerdConfigPaths[configFileVersion][sandboxImagePath],
			setFn: func(value any) (any, error) {
				if criConfig.Containerd == nil {
					return value, nil
				}

				return criConfig.Containerd.SandboxImage, nil
			},
		},
		{
			name: "CNI plugin dir",
			path: containerdConfigPaths[configFileVersion][cniPluginPath],
			setFn: func(_ any) (any, error) {
				return cniPluginDir, nil
			},
		},
	}

	if criConfig.CgroupDriver != nil {
		patches = append(patches, patch{
			name: "cgroup driver",
			path: containerdConfigPaths[configFileVersion][cgroupDriverPath],
			setFn: func(_ any) (any, error) {
				return *criConfig.CgroupDriver == extensionsv1alpha1.CgroupDriverSystemd, nil
			},
		})
	}

	if criConfig.Containerd != nil {
		for _, pluginConfig := range criConfig.Containerd.Plugins {
			patches = append(patches, patch{
				name: "plugin configuration",
				path: replacePluginPath(append(structuredmap.Path{"plugins"}, pluginConfig.Path...), pluginPathReplacements, configFileVersion),
				setFn: func(val any) (any, error) {
					switch op := ptr.Deref(pluginConfig.Op, extensionsv1alpha1.AddPluginPathOperation); op {
					case extensionsv1alpha1.AddPluginPathOperation:
						values, ok := val.(map[string]any)
						if !ok || values == nil {
							values = map[string]any{}
						}

						pluginValues := pluginConfig.Values
						// Return unchanged values if plugin values is not set, i.e. only create table.
						if pluginValues == nil {
							return values, nil
						}

						if err := json.Unmarshal(pluginValues.Raw, &values); err != nil {
							return nil, err
						}

						return values, nil
					case extensionsv1alpha1.RemovePluginPathOperation:
						// Return nil if operation is remove, to delete the entire sub-tree.
						return nil, nil
					default:
						return nil, fmt.Errorf("operation %q is not supported", op)
					}
				},
			})
		}
	}

	for _, p := range patches {
		if err := structuredmap.SetMapEntry(content, p.path, p.setFn); err != nil {
			return fmt.Errorf("unable setting %q in containerd config.toml: %w", p.name, err)
		}
	}

	f, err := r.FS.OpenFile(configFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open containerd config.toml: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Error(err, "Failed closing file", "file", f.Name())
		}
	}()

	return toml.NewEncoder(f).Encode(content)
}

func isConfigPathPrefix(path, prefix structuredmap.Path) bool {
	if len(prefix) > len(path) {
		return false
	}

	return slices.Equal(prefix, path[:len(prefix)])
}

func replaceConfigPathPrefix(path, prefix, replace structuredmap.Path) structuredmap.Path {
	// do not perform a replace operation if the search and replace term are the same
	if slices.Equal(replace, prefix) {
		return path
	}

	if !isConfigPathPrefix(path, prefix) {
		return path
	}

	pathStripped := path[len(prefix):]

	return append(replace, pathStripped...)
}

func replacePluginPath(path structuredmap.Path, replacementMap replacementMap, version containerdConfigFileVersion) structuredmap.Path {
	if version != 3 {
		return path
	}

	for find, replace := range replacementMap {
		if isConfigPathPrefix(path, *find) {
			return replaceConfigPathPrefix(path, *find, replace)
		}
	}
	return path
}
