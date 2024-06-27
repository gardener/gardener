// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/structuredmap"
)

// ReconcileContainerdConfig sets required values of the given containerd configuration.
func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, log logr.Logger, criConfig *extensionsv1alpha1.CRIConfig, containerdChanges containerd) error {
	if !extensionsv1alpha1helper.HasContainerdConfiguration(criConfig) {
		return nil
	}

	if err := r.ensureContainerdConfigDirectories(); err != nil {
		return err
	}

	if err := r.ensureContainerdDefaultConfig(ctx); err != nil {
		return err
	}

	if err := r.ensureContainerdEnvironment(); err != nil {
		return err
	}

	if err := r.ensureContainerdConfiguration(criConfig); err != nil {
		return err
	}

	if err := r.ensureContainerdRegistries(ctx, log, containerdChanges.registries.current); err != nil {
		return err
	}

	if err := r.cleanupUnusedContainerdRegistries(log, containerdChanges.registries.deleted); err != nil {
		return err
	}

	return nil
}

const (
	baseDir   = "/etc/containerd"
	certsDir  = baseDir + "/certs.d"
	configDir = baseDir + "/conf.d"
	dropinDir = "/etc/systemd/system/containerd.service.d"
)

func (r *Reconciler) ensureContainerdConfigDirectories() error {
	for _, dir := range []string{
		extensionsv1alpha1.ContainerDRuntimeContainersBinFolder,
		baseDir,
		configDir,
		certsDir,
		dropinDir,
	} {
		err := r.FS.MkdirAll(dir, defaultDirPermissions)
		if err != nil {
			return fmt.Errorf("unable to ensure containerd config directory %q: %w", dir, err)
		}
	}

	return nil
}

const configFile = baseDir + "/config.toml"

// Exec is the execution function to invoke outside binaries. Exposed for testing.
var Exec = func(ctx context.Context, command string, arg ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, arg...).Output()
}

// ensureContainerdDefaultConfig invokes the 'containerd' and saves the resulting default configuration.
func (r *Reconciler) ensureContainerdDefaultConfig(ctx context.Context) error {
	exists, err := r.fileExists(configFile)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	output, err := Exec(ctx, "containerd", "config", "default")
	if err != nil {
		return fmt.Errorf("error creating containerd default config: %w", err)
	}

	return r.FS.WriteFile(configFile, output, 0644)
}

// ensureContainerdEnvironment sets the environment for the 'containerd' service.
func (r *Reconciler) ensureContainerdEnvironment() error {
	var (
		unitDropin = `[Service]
Environment="PATH=` + extensionsv1alpha1.ContainerDRuntimeContainersBinFolder + `:` + os.Getenv("PATH") + `"
`
	)

	containerdEnvFilePath := path.Join(dropinDir, "30-env_config.conf")
	exists, err := r.fileExists(containerdEnvFilePath)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	err = r.FS.WriteFile(containerdEnvFilePath, []byte(unitDropin), 0644)
	if err != nil {
		return fmt.Errorf("unable to write unit dropin: %w", err)
	}

	return nil
}

// ensureContainerdConfiguration sets the configuration for containerd.
func (r *Reconciler) ensureContainerdConfiguration(criConfig *extensionsv1alpha1.CRIConfig) error {
	config, err := r.FS.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("unable to read containerd config.toml: %w", err)
	}

	content := map[string]any{}

	err = toml.Unmarshal(config, &content)
	if err != nil {
		return fmt.Errorf("unable to decode containerd default config: %w", err)
	}

	type (
		patch struct {
			name  string
			path  structuredmap.Path
			setFn structuredmap.SetFn
		}
	)

	patches := []patch{
		{
			name: "registry config path",
			path: structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "registry", "config_path"},
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
			path: structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"},
			setFn: func(value any) (any, error) {
				if criConfig == nil || criConfig.Containerd == nil {
					return value, nil
				}

				return criConfig.Containerd.SandboxImage, nil
			},
		},
	}

	for _, p := range patches {
		content, err = structuredmap.SetMapEntry(content, p.path, p.setFn)
		if err != nil {
			return fmt.Errorf("unable setting %q in containerd config.toml: %w", p.name, err)
		}
	}

	f, err := r.FS.OpenFile(configFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open containerd config.toml: %w", err)
	}
	defer func() {
		err = f.Close()
	}()

	err = toml.NewEncoder(f).Encode(content)
	if err != nil {
		return fmt.Errorf("unable to encode hosts.toml: %w", err)
	}

	return err
}

// ensureContainerdRegistries configures containerd to use the desired image registries.
func (r *Reconciler) ensureContainerdRegistries(ctx context.Context, log logr.Logger, newRegistries []extensionsv1alpha1.RegistryConfig) error {
	var (
		fns        = make([]flow.TaskFn, 0, len(newRegistries))
		httpClient = http.Client{Timeout: 1 * time.Second}
	)

	for _, registryConfig := range newRegistries {
		fns = append(fns, func(ctx context.Context) error {
			baseDir := path.Join(certsDir, registryConfig.Upstream)
			if err := r.FS.MkdirAll(baseDir, defaultDirPermissions); err != nil {
				return fmt.Errorf("unable to ensure registry config base directory: %w", err)
			}

			hostsTomlFilePath := path.Join(baseDir, "hosts.toml")
			exists, err := r.FS.Exists(hostsTomlFilePath)
			if err != nil {
				return fmt.Errorf("unable to check if registry config file exists: %w", err)
			}

			// Check if registry endpoints are reachable if the config is new.
			// This is especially required when registries run within the cluster and during bootstrap,
			// the Kubernetes deployments are not ready yet.
			if !exists && pointer.BoolDeref(registryConfig.ReadinessProbe, false) {
				log.Info("Probing endpoints for image registry", "upstream", registryConfig.Upstream)
				if err := retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
					for _, registryHosts := range registryConfig.Hosts {
						req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryHosts.URL, nil)
						if err != nil {
							return false, fmt.Errorf("failed to construct http request %s for upstream %s: %w", registryHosts.URL, registryConfig.Upstream, err)
						}

						_, err = httpClient.Do(req)
						if err != nil {
							return false, fmt.Errorf("failed to reach registry %s for upstream %s: %w", registryHosts.URL, registryConfig.Upstream, err)
						}
					}
					return true, nil
				}); err != nil {
					return err
				}

				log.Info("Probing endpoints for image registry succeeded", "upstream", registryConfig.Upstream)
			}

			f, err := r.FS.OpenFile(hostsTomlFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("unable to open hosts.toml: %w", err)
			}

			defer func() {
				err = f.Close()
			}()

			type (
				hostConfig struct {
					Capabilities []string `toml:"capabilities,omitempty"`
					CaCerts      []string `toml:"ca,omitempty"`
				}

				config struct {
					Server *string               `toml:"server,omitempty" comment:"managed by gardener-node-agent"`
					Host   map[string]hostConfig `toml:"host,omitempty"`
				}
			)

			content := config{
				Server: registryConfig.Server,
				Host:   map[string]hostConfig{},
			}

			for _, host := range registryConfig.Hosts {
				h := hostConfig{}

				if len(host.Capabilities) > 0 {
					h.Capabilities = host.Capabilities
				}
				if len(host.CACerts) > 0 {
					h.CaCerts = host.CACerts
				}

				content.Host[host.URL] = h
			}

			err = toml.NewEncoder(f).Encode(content)
			if err != nil {
				return fmt.Errorf("unable to encode hosts.toml: %w", err)
			}

			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) cleanupUnusedContainerdRegistries(log logr.Logger, registriesToRemove []extensionsv1alpha1.RegistryConfig) error {
	for _, registryConfig := range registriesToRemove {
		log.Info("Removing obsolete registry directory", "upstream", registryConfig.Upstream)
		if err := r.FS.RemoveAll(path.Join(certsDir, registryConfig.Upstream)); err != nil {
			return fmt.Errorf("failed to cleanup obsolete registry directory: %w", err)
		}
	}

	return nil
}
