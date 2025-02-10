// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml"
	"github.com/spf13/afero"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/structuredmap"
)

// ReconcileContainerdConfig sets required values of the given containerd configuration.
func (r *Reconciler) ReconcileContainerdConfig(ctx context.Context, log logr.Logger, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	if !extensionsv1alpha1helper.HasContainerdConfiguration(osc.Spec.CRIConfig) {
		return nil
	}

	if err := r.ensureContainerdConfigDirectories(); err != nil {
		return fmt.Errorf("failed to ensure containerd config directories: %w", err)
	}

	if err := r.ensureContainerdDefaultConfig(ctx); err != nil {
		return fmt.Errorf("failed to ensure containerd default config: %w", err)
	}

	if err := r.ensureContainerdConfiguration(log, osc.Spec.CRIConfig); err != nil {
		return fmt.Errorf("failed to ensure containerd config: %w", err)
	}

	// Add the containerd drop-in to the OSC to prevent side effects when containerd.service is changed by extensions too.
	addContainerdEnvironmentDropIn(osc)

	return nil
}

// ReconcileContainerdRegistries configures changed registries for containerd and cleans up abandoned ones.
// Registries without readiness probes are added synchronously and related errors are returned immediately.
// Registries with configured readiness probes are added asynchronously and must be waited for by invoking the returned function.
func (r *Reconciler) ReconcileContainerdRegistries(ctx context.Context, log logr.Logger, changes *operatingSystemConfigChanges) (func() error, error) {
	errChan := r.ensureContainerdRegistries(ctx, log, changes)

	select {
	case err := <-errChan:
		// Return immediately if a result was already sent to the err channel.
		// Note: err can still be nil here, thus the cleanup function call must be returned.
		return func() error {
			return r.cleanupUnusedContainerdRegistries(log, changes)
		}, err
	default:
		return func() error {
			log.Info("Waiting for registries with readiness probes to finish")
			if err := <-errChan; err != nil {
				return err
			}
			return r.cleanupUnusedContainerdRegistries(log, changes)
		}, nil
	}
}

// addContainerdEnvironmentDropIn ingests a drop-in to set the environment for the 'containerd' service.
func addContainerdEnvironmentDropIn(osc *extensionsv1alpha1.OperatingSystemConfig) {
	if osc.Spec.CRIConfig == nil {
		return
	}

	unitDropIn := extensionsv1alpha1.DropIn{
		Name: "30-env_config.conf",
		Content: `[Service]
Environment="PATH=` + extensionsv1alpha1.ContainerDRuntimeContainersBinFolder + `:` + os.Getenv("PATH") + `"
`,
	}

	for i, unit := range osc.Spec.Units {
		if unit.Name == v1beta1constants.OperatingSystemConfigUnitNameContainerDService {
			osc.Spec.Units[i].DropIns = append(osc.Spec.Units[i].DropIns, unitDropIn)
			return
		}
	}

	osc.Spec.Units = append(osc.Spec.Units, extensionsv1alpha1.Unit{
		Name:    v1beta1constants.OperatingSystemConfigUnitNameContainerDService,
		DropIns: []extensionsv1alpha1.DropIn{unitDropIn},
	})
}

const (
	baseDir   = "/etc/containerd"
	certsDir  = baseDir + "/certs.d"
	configDir = baseDir + "/conf.d"
)

func (r *Reconciler) ensureContainerdConfigDirectories() error {
	for _, dir := range []string{
		extensionsv1alpha1.ContainerDRuntimeContainersBinFolder,
		baseDir,
		configDir,
		certsDir,
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
				if criConfig.Containerd == nil {
					return value, nil
				}

				return criConfig.Containerd.SandboxImage, nil
			},
		},
	}

	if criConfig.CgroupDriver != nil {
		patches = append(patches, patch{
			name: "cgroup driver",
			path: structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"},
			setFn: func(_ any) (any, error) {
				return *criConfig.CgroupDriver == extensionsv1alpha1.CgroupDriverSystemd, nil
			},
		})
	}

	if criConfig.Containerd != nil {
		for _, pluginConfig := range criConfig.Containerd.Plugins {
			patches = append(patches, patch{
				name: "plugin configuration",
				path: append(structuredmap.Path{"plugins"}, pluginConfig.Path...),
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

// ensureContainerdRegistries configures containerd to use the desired image registries.
func (r *Reconciler) ensureContainerdRegistries(ctx context.Context, log logr.Logger, changes *operatingSystemConfigChanges) <-chan error {
	var (
		errChan = make(chan error, 1)

		registriesWithoutReadiness []extensionsv1alpha1.RegistryConfig
		registriesWithReadiness    []extensionsv1alpha1.RegistryConfig
	)

	for _, registryConfig := range changes.Containerd.Registries.Changed {
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
		if err := changes.completedContainerdRegistriesChanged(registryConfig.Upstream); err != nil {
			errChan <- err
			return errChan
		}
	}

	fns := make([]flow.TaskFn, 0, len(registriesWithReadiness))
	for _, registryConfig := range registriesWithReadiness {
		fns = append(fns, func(ctx context.Context) error {
			if err := addRegistryToContainerdFunc(ctx, log, registryConfig, r.FS); err != nil {
				return err
			}
			return changes.completedContainerdRegistriesChanged(registryConfig.Upstream)
		})
	}

	go func() {
		errChan <- flow.Parallel(fns...)(ctx)
	}()

	return errChan
}

var (
	//go:embed templates/containerd-hosts.toml.tpl
	tplContentContainerdHosts string
	tplContainerdHosts        *template.Template
)

func init() {
	tplContainerdHosts = template.Must(template.
		New(tplContentContainerdHosts).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentContainerdHosts))
}

func addRegistryToContainerdFunc(ctx context.Context, log logr.Logger, registryConfig extensionsv1alpha1.RegistryConfig, fs afero.Afero) error {
	httpClient := http.Client{Timeout: 1 * time.Second}

	baseDir := path.Join(certsDir, registryConfig.Upstream)
	if err := fs.MkdirAll(baseDir, defaultDirPermissions); err != nil {
		return fmt.Errorf("unable to ensure registry config base directory: %w", err)
	}

	// Check if registry endpoints are reachable if the config is new or changed.
	// This is especially required when registries run within the cluster and during bootstrap,
	// the Kubernetes deployments are not ready yet.
	if ptr.Deref(registryConfig.ReadinessProbe, false) {
		log.Info("Probing endpoints for image registry", "upstream", registryConfig.Upstream)
		if err := retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
			for _, registryHost := range registryConfig.Hosts {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryHost.URL, nil)
				if err != nil {
					return false, fmt.Errorf("failed to construct http request %s for upstream %s: %w", registryHost.URL, registryConfig.Upstream, err)
				}

				if len(registryHost.CACerts) > 0 {
					caCertPool := x509.NewCertPool()
					for _, caCert := range registryHost.CACerts {
						if !filepath.IsAbs(caCert) {
							caCert = filepath.Join(baseDir, caCert)
						}
						pemContent, err := fs.ReadFile(caCert)
						if err != nil {
							return false, fmt.Errorf("failed to read ca file %s for host %s and upstream %s: %w", caCert, registryHost.URL, registryConfig.Upstream, err)
						}
						caCertPool.AppendCertsFromPEM(pemContent)
					}
					httpClient.Transport = &http.Transport{
						TLSClientConfig: &tls.Config{
							RootCAs:    caCertPool,
							MinVersion: tls.VersionTLS12,
						},
					}
				}

				_, err = httpClient.Do(req)
				if err != nil {
					return false, fmt.Errorf("failed to reach registry %s for upstream %s: %w", registryHost.URL, registryConfig.Upstream, err)
				}
			}
			return true, nil
		}); err != nil {
			return err
		}

		log.Info("Probing endpoints for image registry succeeded", "upstream", registryConfig.Upstream)
	}

	hostsTomlFilePath := path.Join(baseDir, "hosts.toml")
	f, err := fs.OpenFile(hostsTomlFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open hosts.toml: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Error(err, "Failed closing file", "file", f.Name())
		}
	}()

	var (
		values = map[string]any{
			"server":      ptr.Deref(registryConfig.Server, ""),
			"hostConfigs": make([]any, 0, len(registryConfig.Hosts)),
		}
	)

	for _, host := range registryConfig.Hosts {
		hostConfig := map[string]any{
			"hostURL": host.URL,
			"capabilities": []extensionsv1alpha1.RegistryCapability{
				extensionsv1alpha1.PullCapability,
				extensionsv1alpha1.ResolveCapability,
			},
		}

		if len(host.Capabilities) > 0 {
			hostConfig["capabilities"] = host.Capabilities
		}
		if len(host.CACerts) > 0 {
			hostConfig["ca"] = host.CACerts
		}

		values["hostConfigs"] = append(values["hostConfigs"].([]any), hostConfig)
	}

	if err := tplContainerdHosts.Execute(f, values); err != nil {
		return err
	}
	log.Info("Configured registry config", "upstream", registryConfig.Upstream)
	return nil
}

func (r *Reconciler) cleanupUnusedContainerdRegistries(log logr.Logger, changes *operatingSystemConfigChanges) error {
	for _, registryConfig := range slices.Clone(changes.Containerd.Registries.Deleted) {
		log.Info("Removing obsolete registry directory", "upstream", registryConfig.Upstream)
		if err := r.FS.RemoveAll(path.Join(certsDir, registryConfig.Upstream)); err != nil {
			return fmt.Errorf("failed to cleanup obsolete registry directory: %w", err)
		}
		if err := changes.completedContainerdRegistriesDeleted(registryConfig.Upstream); err != nil {
			return err
		}
	}

	return nil
}
