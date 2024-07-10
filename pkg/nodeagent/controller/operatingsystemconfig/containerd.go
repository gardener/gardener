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
	"github.com/spf13/afero"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/structuredmap"
)

// ReconcileContainerdConfig sets required values of the given containerd configuration.
func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, log logr.Logger, criConfig *extensionsv1alpha1.CRIConfig) error {
	if !extensionsv1alpha1helper.HasContainerdConfiguration(criConfig) {
		return nil
	}

	if err := r.ensureContainerdConfigDirectories(); err != nil {
		return fmt.Errorf("failed to ensure containerd config directories: %w", err)
	}

	if err := r.ensureContainerdDefaultConfig(ctx); err != nil {
		return fmt.Errorf("failed to ensure containerd default config: %w", err)
	}

	if err := r.ensureContainerdEnvironment(); err != nil {
		return fmt.Errorf("failed to ensure containerd environment: %w", err)
	}

	if err := r.ensureContainerdConfiguration(log, criConfig); err != nil {
		return fmt.Errorf("failed to ensure containerd config: %w", err)
	}

	return nil
}

// ReconcileContainerdRegistries configures desired registries for containerd and cleans up abandoned ones.
// Registries without readiness probes are added synchronously and related errors are returned immediately.
// Registries with configured readiness probes are added asynchronously and must be waited for by invoking the returned function.
func (r *Reconciler) ReconcileContainerdRegistries(ctx context.Context, log logr.Logger, containerdChanges containerd) (func() error, error) {
	errChan := r.ensureContainerdRegistries(ctx, log, containerdChanges.registries.desired)

	select {
	// Check for early errors, to return immediately.
	case err := <-errChan:
		return nil, err
	default:
		return func() error {
			log.Info("Waiting for registries with readiness probes to finish")
			if err := <-errChan; err != nil {
				return err
			}
			return r.cleanupUnusedContainerdRegistries(log, containerdChanges.registries.deleted)
		}, nil
	}
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
		if err := r.FS.MkdirAll(dir, defaultDirPermissions); err != nil {
			return fmt.Errorf("failure for directory %q: %w", dir, err)
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
		return err
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
func (r *Reconciler) ensureContainerdConfiguration(log logr.Logger, criConfig *extensionsv1alpha1.CRIConfig) error {
	config, err := r.FS.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("unable to read containerd config.toml: %w", err)
	}

	content := map[string]any{}

	if err = toml.Unmarshal(config, &content); err != nil {
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
		if err := f.Close(); err != nil {
			log.Error(err, "Failed closing file", "file", f.Name())
		}
	}()

	return toml.NewEncoder(f).Encode(content)
}

// ensureContainerdRegistries configures containerd to use the desired image registries.
func (r *Reconciler) ensureContainerdRegistries(ctx context.Context, log logr.Logger, newRegistries []extensionsv1alpha1.RegistryConfig) <-chan error {
	var (
		errChan = make(chan error, 1)

		registriesWithoutReadiness []extensionsv1alpha1.RegistryConfig
		registriesWithReadiness    []extensionsv1alpha1.RegistryConfig
	)

	for _, registryConfig := range newRegistries {
		if ptr.Deref(registryConfig.ReadinessProbe, false) {
			registriesWithReadiness = append(registriesWithReadiness, registryConfig)
		} else {
			registriesWithoutReadiness = append(registriesWithoutReadiness, registryConfig)
		}
	}

	// Registries without readiness probes can directly and synchronously be added here
	// since there is no longer blocking operation involved.
	for _, registryConfig := range registriesWithoutReadiness {
		if err := addRegistryToContainerdFunc(ctx, log, registryConfig, r.FS); err != nil {
			errChan <- err
			return errChan
		}
	}

	fns := make([]flow.TaskFn, 0, len(registriesWithReadiness))
	for _, registryConfig := range registriesWithReadiness {
		fns = append(fns, func(ctx context.Context) error {
			return addRegistryToContainerdFunc(ctx, log, registryConfig, r.FS)
		})
	}

	go func() {
		errChan <- flow.Parallel(fns...)(ctx)
	}()

	return errChan
}

func addRegistryToContainerdFunc(ctx context.Context, log logr.Logger, registryConfig extensionsv1alpha1.RegistryConfig, fs afero.Afero) error {
	httpClient := http.Client{Timeout: 1 * time.Second}

	baseDir := path.Join(certsDir, registryConfig.Upstream)
	if err := fs.MkdirAll(baseDir, defaultDirPermissions); err != nil {
		return fmt.Errorf("unable to ensure registry config base directory: %w", err)
	}

	hostsTomlFilePath := path.Join(baseDir, "hosts.toml")
	exists, err := fs.Exists(hostsTomlFilePath)
	if err != nil {
		return fmt.Errorf("unable to check if registry config file exists: %w", err)
	}

	// Check if registry endpoints are reachable if the config is new.
	// This is especially required when registries run within the cluster and during bootstrap,
	// the Kubernetes deployments are not ready yet.
	if !exists && ptr.Deref(registryConfig.ReadinessProbe, false) {
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

	f, err := fs.OpenFile(hostsTomlFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open hosts.toml: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Error(err, "Failed closing file", "file", f.Name())
		}
	}()

	type (
		hostConfig struct {
			Capabilities []extensionsv1alpha1.RegistryCapability `toml:"capabilities,omitempty"`
			CaCerts      []string                                `toml:"ca,omitempty"`
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

	return toml.NewEncoder(f).Encode(content)
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
