// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version/verflag"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/cmd/gardener-node-agent/app/bootstrappers"
	"github.com/gardener/gardener/cmd/utils/initrun"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/bootstrap"
	"github.com/gardener/gardener/pkg/nodeagent/controller"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Name is a const for the name of this component.
const Name = "gardener-node-agent"

// NewCommand creates a new cobra.Command for running gardener-node-agent.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := initrun.InitRun(cmd, opts, Name)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			return run(ctx, cancel, log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	cmd.AddCommand(getBootstrapCommand(opts))
	return cmd
}

func getBootstrapCommand(opts *options) *cobra.Command {
	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := initrun.InitRun(cmd, opts, "gardener-node-init")
			if err != nil {
				return err
			}
			return bootstrap.Bootstrap(cmd.Context(), log, afero.Afero{Fs: afero.NewOsFs()}, dbus.New(log), opts.config.Bootstrap)
		},
	}

	flags := bootstrapCmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return bootstrapCmd
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, cfg *config.NodeAgentConfiguration) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)
	fs := afero.Afero{Fs: afero.NewOsFs()}

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}

	log.Info("Getting rest config")
	var (
		restConfig *rest.Config
		err        error
	)

	var mustBootstrap bool
	if len(cfg.ClientConnection.Kubeconfig) > 0 {
		restConfig, err = kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.ClientConnection, nil, kubernetes.AuthTokenFile)
		if err != nil {
			return fmt.Errorf("failed getting REST config from client connection configuration: %w", err)
		}
	} else {
		var migrateKubeconfig bool
		if features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) {
			restConfig, mustBootstrap, err = getRESTConfig(log, fs, cfg)
			if err != nil {
				log.Info("Failed to get REST config for node-agent. Starting migration from access token kubeconfig")
				migrateKubeconfig = true
			}
			if !migrateKubeconfig {
				log.Info("Deleting obsolete access token file (in case it still exists)")
				if err := fs.Remove(nodeagentv1alpha1.TokenFilePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
					return fmt.Errorf("failed removing access token file: %w", err)
				}
			}
		} else {
			var mustFetchAccessToken bool
			restConfig, mustFetchAccessToken, err = getRESTConfigAccessToken(log, cfg)
			if err != nil {
				log.Info("Failed to get REST config for access token. Starting migration from node-authorizer kubeconfig")
				migrateKubeconfig = true
			}

			if mustFetchAccessToken && !migrateKubeconfig {
				log.Info("Fetching access token")
				if err := fetchAccessToken(ctx, log, restConfig); err != nil {
					return fmt.Errorf("failed fetching access token: %w", err)
				}
			}
		}

		// TODO(oliver-goetz): Remove migration code when NodeAgentAuthorizer feature gate is removed
		// Migration from access token kubeconfig to node-authorizer kubeconfig or vice versa
		if migrateKubeconfig {
			if features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) {
				var mustFetchAccessToken bool
				restConfig, mustFetchAccessToken, err = getRESTConfigAccessToken(log, cfg)
				if err != nil || mustFetchAccessToken {
					return fmt.Errorf("failed getting REST config - migrating kubeconfig failed: %w", err)
				}
			} else {
				restConfig, mustBootstrap, err = getRESTConfig(log, fs, cfg)
				if err != nil || mustBootstrap {
					return fmt.Errorf("failed getting REST config - migrating kubeconfig failed: %w", err)
				}
				log.Info("Fetching access token")
				if err := fetchAccessToken(ctx, log, restConfig); err != nil {
					return fmt.Errorf("failed fetching access token: %w", err)
				}
				var mustFetchAccessToken bool
				restConfig, mustFetchAccessToken, err = getRESTConfigAccessToken(log, cfg)
				if err != nil || mustFetchAccessToken {
					return fmt.Errorf("failed getting REST config - migrating kubeconfig failed: %w", err)
				}
				log.Info("Deleting obsolete node-authorizer kubeconfig")
				if err := fs.Remove(nodeagentv1alpha1.KubeconfigFilePath); err != nil {
					return fmt.Errorf("failed removing kubeconfig file: %w", err)
				}
			}
			log.Info("Migrating kubeconfig complete")
		}
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && cfg.Debugging.EnableProfiling {
		extraHandlers = routes.ProfilingHandlers
		if cfg.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Fetching hostname")
	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return fmt.Errorf("failed fetching hostname: %w", err)
	}
	log.Info("Fetched hostname", "hostname", hostName)

	var nodeName string
	if !mustBootstrap {
		log.Info("Fetching name of node (if already registered)")
		nodeName, err = fetchNodeName(ctx, restConfig, hostName)
		if err != nil {
			return fmt.Errorf("failed fetching name of node: %w", err)
		}
	}

	var (
		nodeCacheOptions  = cache.ByObject{Label: labels.SelectorFromSet(labels.Set{corev1.LabelHostname: hostName})}
		leaseCacheOptions = cache.ByObject{Namespaces: map[string]cache.Config{metav1.NamespaceSystem: {}}}
	)

	if nodeName != "" {
		log.Info("Node already registered, found name", "nodeName", nodeName)
		nodeCacheOptions.Field = fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: nodeName})
		nodeCacheOptions.Label = nil
		leaseCacheOptions.Field = fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: gardenerutils.NodeAgentLeaseName(nodeName)})
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Logger:                  log,
		Scheme:                  kubernetes.SeedScheme,
		GracefulShutdownTimeout: ptr.To(5 * time.Second),

		HealthProbeBindAddress: net.JoinHostPort(cfg.Server.HealthProbes.BindAddress, strconv.Itoa(cfg.Server.HealthProbes.Port)),
		Metrics: metricsserver.Options{
			BindAddress:   net.JoinHostPort(cfg.Server.Metrics.BindAddress, strconv.Itoa(cfg.Server.Metrics.Port)),
			ExtraHandlers: extraHandlers,
		},

		Cache: cache.Options{ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {
				Namespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
				Field:      fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: cfg.Controllers.OperatingSystemConfig.SecretName}),
			},
			&corev1.Node{}:          nodeCacheOptions,
			&coordinationv1.Lease{}: leaseCacheOptions,
		}},
		LeaderElection: false,
	})
	if err != nil {
		return err
	}

	log.Info("Setting up health check endpoints")
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}

	log.Info("Creating directory for temporary files", "path", nodeagentv1alpha1.TempDir)
	if err := fs.MkdirAll(nodeagentv1alpha1.TempDir, os.ModeDir); err != nil {
		return fmt.Errorf("unable to create directory for temporary files %q: %w", nodeagentv1alpha1.TempDir, err)
	}

	bootstrapRunnables := []manager.Runnable{
		&bootstrappers.KubeletBootstrapKubeconfig{Log: log.WithName("kubelet-bootstrap-kubeconfig-creator"), FS: fs, APIServerConfig: cfg.APIServer},
	}

	var machineName string
	if features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) {
		machineName, err = fetchMachineNameFromFile(fs)
		// TODO(oliver-goetz): Fetching the machine name from node label is migration code. Remove this migration code when feature gate NodeAgentAuthorizer is removed.
		// Fetching the machine name from node label is for migrating existing nodes when the feature gate is activated, because there is no file with the machine name yet.
		if errors.Is(err, afero.ErrFileNotFound) {
			machineName, err = fetchAndStoreMachineNameFromNode(ctx, restConfig, fs, nodeName)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		bootstrapRunnables = append(bootstrapRunnables,
			&bootstrappers.NodeAgentKubeconfig{
				Log:         log.WithName("nodeagent-kubeconfig-creator"),
				FS:          fs,
				Cancel:      cancel,
				MachineName: machineName,
				Config:      restConfig,
			},
		)
	}

	log.Info("Adding runnables to manager")
	if err := mgr.Add(&controllerutils.ControlledRunner{
		Manager:            mgr,
		BootstrapRunnables: bootstrapRunnables,
		ActualRunnables: []manager.Runnable{
			manager.RunnableFunc(func(ctx context.Context) error {
				return controller.AddToManager(ctx, cancel, mgr, cfg, hostName, machineName, nodeName)
			}),
		},
	}); err != nil {
		return fmt.Errorf("failed adding runnables to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func getRESTConfig(log logr.Logger, fs afero.Afero, cfg *config.NodeAgentConfiguration) (*rest.Config, bool, error) {
	if kubeconfig, err := fs.ReadFile(nodeagentv1alpha1.KubeconfigFilePath); err == nil {
		log.Info("Kubeconfig file exists, using it")
		restConfig, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
		return restConfig, false, err
	} else if !errors.Is(err, afero.ErrFileNotFound) {
		return nil, false, err
	}

	restConfig := &rest.Config{
		Burst: int(cfg.ClientConnection.Burst),
		QPS:   cfg.ClientConnection.QPS,
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: cfg.ClientConnection.AcceptContentTypes,
			ContentType:        cfg.ClientConnection.ContentType,
		},
		Host:            cfg.APIServer.Server,
		TLSClientConfig: rest.TLSClientConfig{CAData: cfg.APIServer.CABundle},
		BearerTokenFile: nodeagentv1alpha1.BootstrapTokenFilePath,
	}

	if ok, err := fs.Exists(nodeagentv1alpha1.BootstrapTokenFilePath); err != nil {
		return nil, false, fmt.Errorf("failed checking whether bootstrap token file %q exists: %w", nodeagentv1alpha1.BootstrapTokenFilePath, err)
	} else if ok {
		log.Info("Kubeconfig file does not exist, but bootstrap token file does - using it", "path", nodeagentv1alpha1.BootstrapTokenFilePath)
		return restConfig, true, nil
	}

	return nil, false, fmt.Errorf("unable to construct REST config (neither kubeconfig file %q nor bootstrap token file %q exist)", nodeagentv1alpha1.TokenFilePath, nodeagentv1alpha1.BootstrapTokenFilePath)
}

func getRESTConfigAccessToken(log logr.Logger, cfg *config.NodeAgentConfiguration) (*rest.Config, bool, error) {
	restConfig := &rest.Config{
		Burst: int(cfg.ClientConnection.Burst),
		QPS:   cfg.ClientConnection.QPS,
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: cfg.ClientConnection.AcceptContentTypes,
			ContentType:        cfg.ClientConnection.ContentType,
		},
		Host:            cfg.APIServer.Server,
		TLSClientConfig: rest.TLSClientConfig{CAData: cfg.APIServer.CABundle},
		BearerTokenFile: nodeagentv1alpha1.TokenFilePath,
	}

	if _, err := os.Stat(restConfig.BearerTokenFile); err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed checking whether token file %q exists: %w", restConfig.BearerTokenFile, err)
	} else if err == nil {
		log.Info("Token file already exists, nothing to be done", "path", restConfig.BearerTokenFile)
		return restConfig, false, nil
	}

	if _, err := os.Stat(nodeagentv1alpha1.BootstrapTokenFilePath); err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed checking whether bootstrap token file %q exists: %w", nodeagentv1alpha1.BootstrapTokenFilePath, err)
	} else if err == nil {
		log.Info("Token file does not exist, but bootstrap token file does - using it", "path", nodeagentv1alpha1.BootstrapTokenFilePath)
		restConfig.BearerTokenFile = nodeagentv1alpha1.BootstrapTokenFilePath
		return restConfig, true, nil
	}

	return nil, false, fmt.Errorf("unable to construct REST config (neither token file %q nor bootstrap token file %q exist)", nodeagentv1alpha1.TokenFilePath, nodeagentv1alpha1.BootstrapTokenFilePath)
}

func fetchAccessToken(ctx context.Context, log logr.Logger, restConfig *rest.Config) error {
	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("unable to create client with bootstrap token: %w", err)
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeagentv1alpha1.AccessSecretName, Namespace: metav1.NamespaceSystem}}
	log.Info("Reading access token secret", "secret", client.ObjectKeyFromObject(secret))
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return fmt.Errorf("failed fetching access token from API server: %w", err)
	}

	token := secret.Data[resourcesv1alpha1.DataKeyToken]
	if len(token) == 0 {
		return fmt.Errorf("secret key %q does not exist or empty", resourcesv1alpha1.DataKeyToken)
	}

	log.Info("Writing downloaded access token to disk", "path", nodeagentv1alpha1.TokenFilePath)
	if err := os.MkdirAll(filepath.Dir(nodeagentv1alpha1.TokenFilePath), fs.ModeDir); err != nil {
		return fmt.Errorf("unable to create directory %q: %w", filepath.Dir(nodeagentv1alpha1.TokenFilePath), err)
	}
	if err := os.WriteFile(nodeagentv1alpha1.TokenFilePath, token, 0600); err != nil {
		return fmt.Errorf("unable to write access token to %s: %w", nodeagentv1alpha1.TokenFilePath, err)
	}

	log.Info("Token written to disk")
	restConfig.BearerTokenFile = nodeagentv1alpha1.TokenFilePath
	return nil
}

func fetchMachineNameFromFile(fs afero.Afero) (string, error) {
	machineName, err := fs.ReadFile(nodeagentv1alpha1.MachineNameFilePath)
	if err != nil {
		return "", fmt.Errorf("failed reading machine-name file %q: %w", nodeagentv1alpha1.MachineNameFilePath, err)
	}
	return strings.Split(string(machineName), "\n")[0], nil
}

func fetchAndStoreMachineNameFromNode(ctx context.Context, restConfig *rest.Config, fs afero.Afero, nodeName string) (string, error) {
	if nodeName == "" {
		return "", fmt.Errorf("node name is empty, cannot fetch machine name from node")
	}
	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return "", fmt.Errorf("unable to create client: %w", err)
	}
	node := &corev1.Node{}
	if err := c.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		return "", fmt.Errorf("unable to fetch node %q: %w", nodeName, err)
	}
	machineName, found := node.Labels[machineutils.MachineLabelKey]
	if !found {
		return "", fmt.Errorf("unable to get machine name. No %q label on node %q", machineutils.MachineLabelKey, node.Name)
	}

	if err := fs.WriteFile(nodeagentv1alpha1.MachineNameFilePath, []byte(machineName), 0600); err != nil {
		return "", fmt.Errorf("error writing machine name to file %q: %w", nodeName, err)
	}

	return machineName, nil
}

func fetchNodeName(ctx context.Context, restConfig *rest.Config, hostName string) (string, error) {
	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return "", fmt.Errorf("unable to create client: %w", err)
	}

	node, err := nodeagent.FetchNodeByHostName(ctx, c, hostName)
	if err != nil {
		return "", err
	}

	if node == nil {
		return "", nil
	}

	return node.Name, nil
}
