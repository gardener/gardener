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
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version/verflag"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	controllerwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/gardener/gardener/cmd/utils/initrun"
	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/bootstrappers"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook"
)

// Name is a const for the name of this component.
const Name = "gardener-resource-manager"

// NewCommand creates a new cobra.Command for running gardener-resource-manager.
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
			return run(cmd.Context(), log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return cmd
}

func run(ctx context.Context, log logr.Logger, cfg *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration) error {
	log.Info("Getting rest configs")
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.SourceClientConnection.Kubeconfig = kubeconfig
	}

	sourceRESTConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.SourceClientConnection.ClientConnectionConfiguration, nil, kubernetes.AuthTokenFile)
	if err != nil {
		return err
	}

	var (
		targetRESTConfig *rest.Config
		managerScheme    = resourcemanagerclient.CombinedScheme
	)

	if cfg.TargetClientConnection != nil {
		if kubeconfig := os.Getenv("TARGET_KUBECONFIG"); kubeconfig != "" {
			cfg.TargetClientConnection.Kubeconfig = kubeconfig
		}

		var err error
		targetRESTConfig, err = kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.TargetClientConnection.ClientConnectionConfiguration, nil, kubernetes.AuthTokenFile)
		if err != nil {
			return err
		}

		managerScheme = resourcemanagerclient.SourceScheme
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && ptr.Deref(cfg.Debugging.EnableProfiling, false) {
		extraHandlers = routes.ProfilingHandlers
		if ptr.Deref(cfg.Debugging.EnableContentionProfiling, false) {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(sourceRESTConfig, manager.Options{
		Logger:                  log,
		Scheme:                  managerScheme,
		GracefulShutdownTimeout: ptr.To(5 * time.Second),
		Cache: cache.Options{
			DefaultNamespaces: getCacheConfig(cfg.SourceClientConnection.Namespaces),
			SyncPeriod:        &cfg.SourceClientConnection.CacheResyncPeriod.Duration,
		},
		HealthProbeBindAddress: net.JoinHostPort(cfg.Server.HealthProbes.BindAddress, strconv.Itoa(cfg.Server.HealthProbes.Port)),
		Metrics: metricsserver.Options{
			BindAddress:   net.JoinHostPort(cfg.Server.Metrics.BindAddress, strconv.Itoa(cfg.Server.Metrics.Port)),
			ExtraHandlers: extraHandlers,
		},

		LeaderElection:                *cfg.LeaderElection.LeaderElect,
		LeaderElectionResourceLock:    cfg.LeaderElection.ResourceLock,
		LeaderElectionID:              cfg.LeaderElection.ResourceName,
		LeaderElectionNamespace:       cfg.LeaderElection.ResourceNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &cfg.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:                 &cfg.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                   &cfg.LeaderElection.RetryPeriod.Duration,

		WebhookServer: controllerwebhook.NewServer(controllerwebhook.Options{
			Host:    cfg.Server.Webhooks.BindAddress,
			Port:    cfg.Server.Webhooks.Port,
			CertDir: cfg.Server.Webhooks.TLS.ServerCertDir,
		}),
	})
	if err != nil {
		return err
	}

	log.Info("Setting up health check endpoints")
	sourceClientSet, err := kubernetesclientset.NewForConfig(sourceRESTConfig)
	if err != nil {
		return fmt.Errorf("could not create clientset for source cluster: %+v", err)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("apiserver-healthz", gardenerhealthz.NewAPIServerHealthz(ctx, sourceClientSet.RESTClient())); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("source-informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, mgr.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("source-informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return err
	}

	var targetCluster cluster.Cluster = mgr
	if targetRESTConfig != nil {
		log.Info("Setting up cluster object for target")
		targetCluster, err = cluster.New(targetRESTConfig, func(opts *cluster.Options) {
			opts.Scheme = resourcemanagerclient.TargetScheme
			opts.Logger = log

			// use dynamic rest mapper for target cluster, which will automatically rediscover resources on NoMatchErrors
			// but is rate-limited to not issue to many discovery calls (rate-limit shared across all reconciliations)
			opts.MapperProvider = apiutil.NewDynamicRESTMapper

			if cfg.Webhooks.NodeAgentAuthorizer.Enabled {
				opts.Cache.ByObject = map[client.Object]cache.ByObject{
					// Needed for node-agent-authorizer webhook
					&corev1.Pod{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
				}
			}

			opts.Cache.DefaultNamespaces = getCacheConfig(cfg.TargetClientConnection.Namespaces)
			opts.Cache.SyncPeriod = &cfg.TargetClientConnection.CacheResyncPeriod.Duration

			opts.Client.Cache = &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.Event{},
					&eventsv1beta1.Event{},
					&eventsv1.Event{},
				},
			}
		})
		if err != nil {
			return fmt.Errorf("could not instantiate target cluster: %w", err)
		}

		log.Info("Setting up checks for target informer sync")
		if err := mgr.AddHealthzCheck("target-informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, targetCluster.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
			return err
		}
		if err := mgr.AddReadyzCheck("target-informer-sync", gardenerhealthz.NewCacheSyncHealthz(targetCluster.GetCache())); err != nil {
			return err
		}

		log.Info("Adding target cluster to manager")
		if err := mgr.Add(targetCluster); err != nil {
			return fmt.Errorf("failed adding target cluster to manager: %w", err)
		}
	}

	log.Info("Adding field indexes to informers")
	if err := addAllFieldIndexes(ctx, targetCluster.GetFieldIndexer()); err != nil {
		return fmt.Errorf("failed adding indexes: %w", err)
	}

	log.Info("Adding webhook handlers to manager")
	if err := webhook.AddToManager(mgr, mgr, targetCluster, cfg); err != nil {
		return fmt.Errorf("failed adding webhook handlers to manager: %w", err)
	}

	log.Info("Adding controllers to manager")
	if err := mgr.Add(&controllerutils.ControlledRunner{
		Manager:            mgr,
		BootstrapRunnables: []manager.Runnable{&bootstrappers.IdentityDeterminer{Logger: log, SourceClient: mgr.GetClient(), Config: cfg}},
		ActualRunnables:    []manager.Runnable{manager.RunnableFunc(func(context.Context) error { return controller.AddToManager(ctx, mgr, mgr, targetCluster, cfg) })},
	}); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func addAllFieldIndexes(ctx context.Context, i client.FieldIndexer) error {
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

func getCacheConfig(namespaces []string) map[string]cache.Config {
	cacheConfig := map[string]cache.Config{}

	for _, namespace := range namespaces {
		cacheConfig[namespace] = cache.Config{}
	}

	return cacheConfig
}
