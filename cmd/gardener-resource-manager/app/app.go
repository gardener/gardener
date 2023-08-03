// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/cmd/gardener-resource-manager/app/bootstrappers"
	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook"
	thirdpartyapiutil "github.com/gardener/gardener/third_party/controller-runtime/pkg/apiutil"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.complete(); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}

			log, err := logger.NewZapLogger(opts.config.LogLevel, opts.config.LogFormat)
			if err != nil {
				return fmt.Errorf("error instantiating zap logger: %w", err)
			}

			logf.SetLogger(log)
			klog.SetLogger(log)

			log.Info("Starting "+Name, "version", version.Get())
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value)) //nolint:logcheck
			})

			// don't output usage on further errors raised during execution
			cmd.SilenceUsage = true
			// further errors will be logged properly, don't duplicate
			cmd.SilenceErrors = true

			return run(cmd.Context(), log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return cmd
}

func run(ctx context.Context, log logr.Logger, cfg *config.ResourceManagerConfiguration) error {
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

	log.Info("Setting up manager")
	mgr, err := manager.New(sourceRESTConfig, manager.Options{
		Logger:                  log,
		Scheme:                  managerScheme,
		GracefulShutdownTimeout: pointer.Duration(5 * time.Second),
		Namespace:               *cfg.SourceClientConnection.Namespace,
		SyncPeriod:              &cfg.SourceClientConnection.CacheResyncPeriod.Duration,

		Host:                   cfg.Server.Webhooks.BindAddress,
		Port:                   cfg.Server.Webhooks.Port,
		CertDir:                cfg.Server.Webhooks.TLS.ServerCertDir,
		HealthProbeBindAddress: net.JoinHostPort(cfg.Server.HealthProbes.BindAddress, strconv.Itoa(cfg.Server.HealthProbes.Port)),
		MetricsBindAddress:     net.JoinHostPort(cfg.Server.Metrics.BindAddress, strconv.Itoa(cfg.Server.Metrics.Port)),

		LeaderElection:                cfg.LeaderElection.LeaderElect,
		LeaderElectionResourceLock:    cfg.LeaderElection.ResourceLock,
		LeaderElectionID:              cfg.LeaderElection.ResourceName,
		LeaderElectionNamespace:       cfg.LeaderElection.ResourceNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &cfg.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:                 &cfg.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                   &cfg.LeaderElection.RetryPeriod.Duration,
		Controller: controllerconfig.Controller{
			RecoverPanic: pointer.Bool(true),
		},
	})
	if err != nil {
		return err
	}

	if cfg.Debugging != nil && cfg.Debugging.EnableProfiling {
		if err := (routes.Profiling{}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding profiling handlers to manager: %w", err)
		}
		if cfg.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
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
			opts.MapperProvider = func(config *rest.Config, httpClient *http.Client) (meta.RESTMapper, error) {
				return thirdpartyapiutil.NewDynamicRESTMapper(
					config,
					thirdpartyapiutil.WithLazyDiscovery,
					thirdpartyapiutil.WithLimiter(rate.NewLimiter(rate.Every(1*time.Minute), 1)), // rediscover at maximum every minute
				)
			}

			opts.Cache.Namespaces = []string{*cfg.TargetClientConnection.Namespace}
			opts.SyncPeriod = &cfg.TargetClientConnection.CacheResyncPeriod.Duration

			if *cfg.TargetClientConnection.DisableCachedClient {
				opts.NewClient = func(config *rest.Config, opts client.Options) (client.Client, error) {
					return client.New(config, opts)
				}
			}

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

		log.Info("Setting up ready check for target informer sync")
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
