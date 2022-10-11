// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"os"
	goruntime "runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/cmd/gardenlet/app/bootstrappers"
	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	"github.com/gardener/gardener/pkg/gardenlet/controller"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Name is a const for the name of this component.
const Name = "gardenlet"

// NewCommand creates a new cobra.Command for running gardenlet.
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

			ctx, cancel := context.WithCancel(cmd.Context())
			return run(ctx, cancel, log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return cmd
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, cfg *config.GardenletConfiguration) error {
	// Add feature flags
	if err := gardenletfeatures.FeatureGate.SetFromMap(cfg.FeatureGates); err != nil {
		return err
	}
	log.Info("Feature Gates", "featureGates", gardenletfeatures.FeatureGate.String())

	if gardenletfeatures.FeatureGate.Enabled(features.ReversedVPN) && !gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		return fmt.Errorf("inconsistent feature gate: APIServerSNI is required for ReversedVPN (APIServerSNI: %t, ReversedVPN: %t)",
			gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI), gardenletfeatures.FeatureGate.Enabled(features.ReversedVPN))
	}

	if kubeconfig := os.Getenv("GARDEN_KUBECONFIG"); kubeconfig != "" {
		cfg.GardenClientConnection.Kubeconfig = kubeconfig
	}
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.SeedClientConnection.Kubeconfig = kubeconfig
	}

	log.Info("Getting rest config for seed")
	seedRESTConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.SeedClientConnection.ClientConnectionConfiguration, nil)
	if err != nil {
		return err
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(seedRESTConfig, manager.Options{
		Logger:                  log,
		Scheme:                  kubernetes.SeedScheme,
		HealthProbeBindAddress:  fmt.Sprintf("%s:%d", cfg.Server.HealthProbes.BindAddress, cfg.Server.HealthProbes.Port),
		MetricsBindAddress:      fmt.Sprintf("%s:%d", cfg.Server.Metrics.BindAddress, cfg.Server.Metrics.Port),
		GracefulShutdownTimeout: pointer.Duration(5 * time.Second),

		LeaderElection:             cfg.LeaderElection.LeaderElect,
		LeaderElectionResourceLock: cfg.LeaderElection.ResourceLock,
		LeaderElectionID:           cfg.LeaderElection.ResourceName,
		LeaderElectionNamespace:    cfg.LeaderElection.ResourceNamespace,
		LeaseDuration:              &cfg.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &cfg.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &cfg.LeaderElection.RetryPeriod.Duration,
		// TODO: enable this once we have refactored all controllers and added them to this manager
		// LeaderElectionReleaseOnCancel: true,

		ClientDisableCacheFor: []client.Object{
			&corev1.Event{},
			&eventsv1.Event{},
		},
	})
	if err != nil {
		return err
	}

	log.Info("Setting up periodic health manager")
	healthGracePeriod := time.Duration((*cfg.Controllers.Seed.LeaseResyncSeconds)*(*cfg.Controllers.Seed.LeaseResyncMissThreshold)) * time.Second
	healthManager := gardenerhealthz.NewPeriodicHealthz(clock.RealClock{}, healthGracePeriod)
	healthManager.Set(true)

	log.Info("Setting up health check endpoints")
	if err := mgr.AddHealthzCheck("periodic-health", gardenerhealthz.CheckerFunc(healthManager)); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}

	log.Info("Setting up ready check for seed informer sync")
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
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

	log.Info("Adding runnables to manager for bootstrapping")
	kubeconfigBootstrapResult := &bootstrappers.KubeconfigBootstrapResult{}

	if err := mgr.Add(&controllerutils.ControlledRunner{
		Manager: mgr,
		BootstrapRunnables: []manager.Runnable{
			&bootstrappers.SeedConfigChecker{
				SeedClient: mgr.GetClient(),
				SeedConfig: cfg.SeedConfig,
			},
			&bootstrappers.GardenKubeconfig{
				SeedClient: mgr.GetClient(),
				Log:        mgr.GetLogger().WithName("bootstrap"),
				Config:     cfg,
				Result:     kubeconfigBootstrapResult,
			},
		},
		ActualRunnables: []manager.Runnable{
			&garden{
				cancel:                    cancel,
				mgr:                       mgr,
				config:                    cfg,
				healthManager:             healthManager,
				kubeconfigBootstrapResult: kubeconfigBootstrapResult,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed adding runnables to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

type garden struct {
	cancel                    context.CancelFunc
	mgr                       manager.Manager
	config                    *config.GardenletConfiguration
	healthManager             gardenerhealthz.Manager
	kubeconfigBootstrapResult *bootstrappers.KubeconfigBootstrapResult
}

func (g *garden) Start(ctx context.Context) error {
	log := g.mgr.GetLogger()

	log.Info("Getting rest config for garden")
	gardenRESTConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&g.config.GardenClientConnection.ClientConnectionConfiguration, g.kubeconfigBootstrapResult.Kubeconfig)
	if err != nil {
		return err
	}

	log.Info("Setting up cluster object for garden")
	gardenCluster, err := cluster.New(gardenRESTConfig, func(opts *cluster.Options) {
		opts.Scheme = kubernetes.GardenScheme
		opts.Logger = log

		// gardenlet does not have the required RBAC permissions for listing/watching the following resources, so let's
		// prevent any attempts to cache them.
		opts.ClientDisableCacheFor = []client.Object{
			&gardencorev1alpha1.ExposureClass{},
			&gardencorev1alpha1.ShootState{},
			&gardencorev1beta1.CloudProfile{},
			&gardencorev1beta1.ControllerDeployment{},
			&gardencorev1beta1.Project{},
			&gardencorev1beta1.SecretBinding{},
			&certificatesv1.CertificateSigningRequest{},
			&coordinationv1.Lease{},
			&corev1.Namespace{},
			&corev1.ConfigMap{},
			&corev1.Event{},
			&eventsv1.Event{},
		}

		opts.NewCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			// gardenlet should watch only objects which are related to the seed it is responsible for.
			opts.SelectorsByObject = map[client.Object]cache.ObjectSelector{
				&gardencorev1beta1.ControllerInstallation{}: {
					Field: fields.SelectorFromSet(fields.Set{core.SeedRefName: g.config.SeedConfig.SeedTemplate.Name}),
				},
			}

			// gardenlet should watch secrets only in the seed namespace of the seed it is responsible for. We don't use
			// the above selector mechanism here since we want to still fall back to reading secrets with the API reader
			// (i.e., not from cache) in case the respective secret is not found in the cache. This is realized by this
			// aggregator cache we are using here.
			return kubernetes.AggregatorCacheFunc(
				kubernetes.NewRuntimeCache,
				map[client.Object]cache.NewCacheFunc{
					&corev1.Secret{}: cache.MultiNamespacedCacheBuilder([]string{gutil.ComputeGardenNamespace(g.kubeconfigBootstrapResult.SeedName)}),
				},
				kubernetes.GardenScheme,
			)(config, opts)
		}

		// The created multi-namespace cache does not fall back to an uncached reader in case the gardenlet tries to
		// read a secret from another namespace. There might be secrets in namespace other than the seed-specific
		// namespace (e.g., backup secret in the SeedSpec). Hence, let's use a fallback client which falls back to an
		// uncached reader in case it fails to read objects from the cache.
		opts.NewClient = func(cache cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
			uncachedClient, err := client.New(config, options)
			if err != nil {
				return nil, err
			}

			delegatingClient, err := client.NewDelegatingClient(client.NewDelegatingClientInput{
				CacheReader:     cache,
				Client:          uncachedClient,
				UncachedObjects: uncachedObjects,
			})
			if err != nil {
				return nil, err
			}

			return &kubernetes.FallbackClient{
				Client: delegatingClient,
				Reader: uncachedClient,
			}, nil
		}
	})
	if err != nil {
		return fmt.Errorf("failed creating garden cluster object: %w", err)
	}

	log.Info("Setting up ready check for garden informer sync")
	if err := g.mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(gardenCluster.GetCache())); err != nil {
		return err
	}

	log.Info("Cleaning bootstrap authentication data used to request a certificate if needed")
	if len(g.kubeconfigBootstrapResult.CSRName) > 0 && len(g.kubeconfigBootstrapResult.SeedName) > 0 {
		if err := bootstrap.DeleteBootstrapAuth(ctx, gardenCluster.GetClient(), gardenCluster.GetClient(), g.kubeconfigBootstrapResult.CSRName, g.kubeconfigBootstrapResult.SeedName); err != nil {
			return fmt.Errorf("failed cleaning bootstrap auth data: %w", err)
		}
	}

	log.Info("Adding field indexes to informers")
	if err := addAllFieldIndexes(ctx, gardenCluster.GetFieldIndexer()); err != nil {
		return fmt.Errorf("failed adding indexes: %w", err)
	}

	log.Info("Adding garden cluster to manager")
	if err := g.mgr.Add(gardenCluster); err != nil {
		return fmt.Errorf("failed adding garden cluster to manager: %w", err)
	}

	log.Info("Registering Seed object in garden cluster")
	if err := g.registerSeed(ctx, gardenCluster.GetClient()); err != nil {
		return err
	}

	log.Info("Setting up shoot client map")
	shootClientMap, err := clientmapbuilder.
		NewShootClientMapBuilder().
		WithGardenClient(gardenCluster.GetClient()).
		WithSeedClient(g.mgr.GetClient()).
		WithClientConnectionConfig(&g.config.ShootClientConnection.ClientConnectionConfiguration).
		Build(log)
	if err != nil {
		return fmt.Errorf("failed to build shoot ClientMap: %w", err)
	}

	log.Info("Fetching cluster identity and garden namespace from garden cluster")
	configMap := &corev1.ConfigMap{}
	if err := gardenCluster.GetClient().Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ClusterIdentity), configMap); err != nil {
		return fmt.Errorf("failed getting cluster-identity ConfigMap in garden cluster: %w", err)
	}

	gardenClusterIdentity, ok := configMap.Data[v1beta1constants.ClusterIdentity]
	if !ok {
		return fmt.Errorf("cluster-identity ConfigMap data does not have %q key", v1beta1constants.ClusterIdentity)
	}

	// TODO(rfranzke): Move this to the controller.AddControllersToManager function once legacy controllers relying on
	// it have been refactored.
	seedClientSet, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(g.mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(g.mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(g.mgr.GetClient()),
		kubernetes.WithRuntimeCache(g.mgr.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("failed creating seed clientset: %w", err)
	}

	log.Info("Adding runnables now that bootstrapping is finished")
	runnables := []manager.Runnable{
		g.healthManager,
		shootClientMap,
		&controller.LegacyControllerFactory{
			Log:                   log,
			Config:                g.config,
			GardenCluster:         gardenCluster,
			SeedCluster:           g.mgr,
			SeedClientSet:         seedClientSet,
			ShootClientMap:        shootClientMap,
			HealthManager:         g.healthManager,
			GardenClusterIdentity: gardenClusterIdentity,
		},
	}

	if g.config.GardenClientConnection.KubeconfigSecret != nil {
		gardenClientSet, err := kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(gardenCluster.GetConfig()),
			kubernetes.WithRuntimeAPIReader(gardenCluster.GetAPIReader()),
			kubernetes.WithRuntimeClient(gardenCluster.GetClient()),
			kubernetes.WithRuntimeCache(gardenCluster.GetCache()),
		)
		if err != nil {
			return fmt.Errorf("failed creating garden clientset: %w", err)
		}

		certificateManager := certificate.NewCertificateManager(log, gardenClientSet, g.mgr.GetClient(), g.config)

		runnables = append(runnables, manager.RunnableFunc(func(ctx context.Context) error {
			return certificateManager.ScheduleCertificateRotation(ctx, g.cancel, g.mgr.GetEventRecorderFor("certificate-manager"))
		}))
	}

	if err := controllerutils.AddAllRunnables(g.mgr, runnables...); err != nil {
		return err
	}

	log.Info("Adding controllers to manager")
	gardenNamespace := &corev1.Namespace{}
	if err := gardenCluster.GetClient().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gardenNamespace); err != nil {
		return fmt.Errorf("failed getting garden namespace in garden cluster: %w", err)
	}

	if err := controller.AddControllersToManager(
		g.mgr,
		gardenCluster,
		g.mgr,
		seedClientSet,
		g.config,
		gardenNamespace,
		gardenClusterIdentity,
	); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	return nil
}

func (g *garden) registerSeed(ctx context.Context, gardenClient client.Client) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: g.config.SeedConfig.Name,
		},
	}

	// Convert gardenlet config to an external version
	cfg, err := confighelper.ConvertGardenletConfigurationExternal(g.config)
	if err != nil {
		return fmt.Errorf("could not convert gardenlet configuration: %w", err)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, gardenClient, seed, func() error {
		seed.Labels = utils.MergeStringMaps(map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
		}, g.config.SeedConfig.Labels)

		seed.Spec = cfg.SeedConfig.Spec
		return nil
	}); err != nil {
		return fmt.Errorf("could not register seed %q: %w", seed.Name, err)
	}

	// Verify that the gardener-controller-manager has created the seed-specific namespace. Here we also accept
	// 'forbidden' errors since the SeedAuthorizer (if enabled) forbid the gardenlet to read the namespace in case the
	// gardener-controller-manager has not yet created it.
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return wait.PollUntilWithContext(timeoutCtx, 500*time.Millisecond, func(context.Context) (done bool, err error) {
		if err := gardenClient.Get(ctx, kutil.Key(gutil.ComputeGardenNamespace(g.config.SeedConfig.Name)), &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

func addAllFieldIndexes(ctx context.Context, i client.FieldIndexer) error {
	for _, fn := range []func(context.Context, client.FieldIndexer) error{
		// core API group
		indexer.AddShootSeedName,
		indexer.AddShootStatusSeedName,
		indexer.AddBackupBucketSeedName,
		indexer.AddBackupEntrySeedName,
		indexer.AddControllerInstallationSeedRefName,
		indexer.AddControllerInstallationRegistrationRefName,
		// operations API group
		indexer.AddBastionShootName,
		// seedmanagement API group
		indexer.AddManagedSeedShootName,
	} {
		if err := fn(ctx, i); err != nil {
			return err
		}
	}

	return nil
}
