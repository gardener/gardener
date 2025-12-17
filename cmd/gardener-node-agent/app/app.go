// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

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
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/cmd/utils/initrun"
	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/bootstrap"
	"github.com/gardener/gardener/pkg/nodeagent/bootstrappers"
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
			return run(ctx, cancel, log, opts.config, opts.configDir)
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

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, cfg *nodeagentconfigv1alpha1.NodeAgentConfiguration, cfgDir string) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)
	fs := afero.Afero{Fs: afero.NewOsFs()}

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}

	log.Info("Getting rest config")
	restConfig, err := getRESTConfig(ctx, log, fs, cfg)
	if err != nil {
		return fmt.Errorf("failed getting REST config: %w", err)
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && ptr.Deref(cfg.Debugging.EnableProfiling, false) {
		extraHandlers = routes.ProfilingHandlers
		if ptr.Deref(cfg.Debugging.EnableContentionProfiling, false) {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Fetching hostname")
	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return fmt.Errorf("failed fetching hostname: %w", err)
	}
	log.Info("Fetched hostname", "hostname", hostName)

	log.Info("Fetching name of node (if already registered)")
	nodeName, err := fetchNodeName(ctx, restConfig, hostName)
	if err != nil {
		return fmt.Errorf("failed fetching name of node: %w", err)
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
		Cache:          cache.Options{ByObject: getCache(log, hostName, nodeName, cfg.Controllers.OperatingSystemConfig.SecretName)},
		LeaderElection: false,
	})
	if err != nil {
		return err
	}

	log.Info("Setting up health check endpoints")
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, mgr.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}

	log.Info("Creating directory for temporary files", "path", nodeagentconfigv1alpha1.TempDir)
	if err := fs.MkdirAll(nodeagentconfigv1alpha1.TempDir, os.ModeDir); err != nil {
		return fmt.Errorf("unable to create directory for temporary files %q: %w", nodeagentconfigv1alpha1.TempDir, err)
	}

	log.Info("Adding field indexes to informers")
	if err := addAllFieldIndexes(ctx, mgr.GetFieldIndexer(), nodeName); err != nil {
		return fmt.Errorf("failed adding indexes: %w", err)
	}

	machineName, err := fetchMachineNameFromFile(fs)
	if err != nil {
		return fmt.Errorf("failed fetching machine name from file: %w", err)
	}

	log.Info("Adding runnables to manager")

	if err := mgr.Add(&controllerutils.ControlledRunner{
		Manager: mgr,
		BootstrapRunnables: []manager.Runnable{
			&bootstrappers.KubeletBootstrapKubeconfig{
				Log:             log.WithName("kubelet-bootstrap-kubeconfig-creator"),
				FS:              fs,
				APIServerConfig: cfg.APIServer,
			},
			&bootstrappers.OSCChecker{
				Log: log.WithName("bootstrapper-file-checker"),
				FS:  fs,
			},
		},
		ActualRunnables: []manager.Runnable{
			manager.RunnableFunc(func(ctx context.Context) error {
				return controller.AddToManager(ctx, cancel, mgr, cfg, hostName, machineName, nodeName, cfgDir)
			}),
		},
	}); err != nil {
		return fmt.Errorf("failed adding runnables to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func getCache(log logr.Logger, hostName, nodeName, secretName string) map[client.Object]cache.ByObject {
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

	out := map[client.Object]cache.ByObject{
		&corev1.Secret{}: {
			Namespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
			Field:      fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: secretName}),
		},
		&corev1.Node{}:          nodeCacheOptions,
		&coordinationv1.Lease{}: leaseCacheOptions,
	}

	if nodeName != "" {
		out[&corev1.Pod{}] = cache.ByObject{Field: fields.OneTermEqualSelector(indexer.PodNodeName, nodeName)}
	}

	return out
}

func getRESTConfig(ctx context.Context, log logr.Logger, fs afero.Afero, cfg *nodeagentconfigv1alpha1.NodeAgentConfiguration) (*rest.Config, error) {
	if len(cfg.ClientConnection.Kubeconfig) > 0 {
		log.Info("Creating REST config from client configuration")
		restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.ClientConnection, nil, kubernetes.AuthTokenFile)
		if err != nil {
			return nil, fmt.Errorf("failed getting REST config from client connection configuration: %w", err)
		}
		return restConfig, nil
	}

	log.Info("Creating REST config for node-agent-authorizer")

	return getRESTConfigNodeAgentAuthorizer(ctx, log, fs, cfg)
}

func getRESTConfigNodeAgentAuthorizer(ctx context.Context, log logr.Logger, fs afero.Afero, cfg *nodeagentconfigv1alpha1.NodeAgentConfiguration) (*rest.Config, error) {
	if kubeconfigExists, err := fs.Exists(nodeagentconfigv1alpha1.KubeconfigFilePath); err != nil {
		return nil, fmt.Errorf("failed checking whether kubeconfig file %q exists: %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	} else if !kubeconfigExists {
		restConfig := &rest.Config{
			Burst: int(cfg.ClientConnection.Burst),
			QPS:   cfg.ClientConnection.QPS,
			ContentConfig: rest.ContentConfig{
				AcceptContentTypes: cfg.ClientConnection.AcceptContentTypes,
				ContentType:        cfg.ClientConnection.ContentType,
			},
			Host:            cfg.APIServer.Server,
			TLSClientConfig: rest.TLSClientConfig{CAData: cfg.APIServer.CABundle},
			BearerTokenFile: nodeagentconfigv1alpha1.BootstrapTokenFilePath,
		}

		if bootstrapTokenExists, err := fs.Exists(nodeagentconfigv1alpha1.BootstrapTokenFilePath); err != nil {
			return nil, fmt.Errorf("failed checking whether bootstrap token file %q exists: %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
		} else if !bootstrapTokenExists {
			return nil, fmt.Errorf("unable to construct REST config (neither kubeconfig file %q nor bootstrap token file %q exist)", nodeagentconfigv1alpha1.TokenFilePath, nodeagentconfigv1alpha1.BootstrapTokenFilePath)
		}
		log.Info("Kubeconfig file does not exist, but bootstrap token file does - using it to request certificate", "path", nodeagentconfigv1alpha1.BootstrapTokenFilePath)

		machineName, err := fetchMachineNameFromFile(fs)
		if err != nil {
			return nil, fmt.Errorf("failed fetching machine name from file: %w", err)
		}

		if err := nodeagent.RequestAndStoreKubeconfig(ctx, log, fs, restConfig, machineName); err != nil {
			return nil, fmt.Errorf("failed requesting and storing kubeconfig: %w", err)
		}
	}

	kubeconfig, err := fs.ReadFile(nodeagentconfigv1alpha1.KubeconfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig file %q: %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	}

	log.Info("Kubeconfig file exists, using it")
	restConfig, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating REST config from kubeconfig file %q: %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	}

	return restConfig, nil
}

func fetchMachineNameFromFile(fs afero.Afero) (string, error) {
	machineName, err := fs.ReadFile(nodeagentconfigv1alpha1.MachineNameFilePath)
	if err != nil {
		return "", fmt.Errorf("failed reading machine-name file %q: %w", nodeagentconfigv1alpha1.MachineNameFilePath, err)
	}
	return strings.Split(string(machineName), "\n")[0], nil
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

func addAllFieldIndexes(ctx context.Context, i client.FieldIndexer, nodeName string) error {
	if nodeName == "" {
		return nil
	}

	for _, fn := range []func(context.Context, client.FieldIndexer) error{
		// core/v1 API group
		indexer.AddPodNodeName,
	} {
		if err := fn(ctx, i); err != nil {
			return err
		}
	}

	return nil
}
