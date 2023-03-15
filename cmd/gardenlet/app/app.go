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
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	controllerconfigv1alpha1 "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/cmd/gardenlet/app/bootstrappers"
	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	"github.com/gardener/gardener/pkg/gardenlet/controller"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
		GracefulShutdownTimeout: pointer.Duration(5 * time.Second),

		HealthProbeBindAddress: fmt.Sprintf("%s:%d", cfg.Server.HealthProbes.BindAddress, cfg.Server.HealthProbes.Port),
		MetricsBindAddress:     fmt.Sprintf("%s:%d", cfg.Server.Metrics.BindAddress, cfg.Server.Metrics.Port),

		LeaderElection:                cfg.LeaderElection.LeaderElect,
		LeaderElectionResourceLock:    cfg.LeaderElection.ResourceLock,
		LeaderElectionID:              cfg.LeaderElection.ResourceName,
		LeaderElectionNamespace:       cfg.LeaderElection.ResourceNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &cfg.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:                 &cfg.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                   &cfg.LeaderElection.RetryPeriod.Duration,
		Controller: controllerconfigv1alpha1.ControllerConfigurationSpec{
			RecoverPanic: pointer.Bool(true),
		},

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
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("periodic-health", gardenerhealthz.CheckerFunc(healthManager)); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("seed-informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
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

			// TODO(rfranzke): Remove this in a future version.
			// Ensure all existing ETCD encryption secrets get the 'garbage-collectable' label. There was a bug which
			// prevented this from happening, see https://github.com/gardener/gardener/pull/7244.
			manager.RunnableFunc(func(ctx context.Context) error {
				secretList := &corev1.SecretList{}
				if err := mgr.GetClient().List(ctx, secretList, client.MatchingLabels{v1beta1constants.LabelRole: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration}); err != nil {
					return err
				}

				var tasks []flow.TaskFn

				for _, obj := range secretList.Items {
					secret := obj

					tasks = append(tasks, func(ctx context.Context) error {
						patch := client.MergeFrom(secret.DeepCopy())
						metav1.SetMetaDataLabel(&secret.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)
						return mgr.GetClient().Patch(ctx, &secret, patch)
					})
				}

				return flow.Parallel(tasks...)(ctx)
			}),
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

		opts.ClientDisableCacheFor = []client.Object{
			&corev1.Event{},
			&eventsv1.Event{},
		}

		opts.NewCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			// gardenlet should watch only objects which are related to the seed it is responsible for.
			opts.SelectorsByObject = map[client.Object]cache.ObjectSelector{
				&gardencorev1beta1.ControllerInstallation{}: {
					Field: fields.SelectorFromSet(fields.Set{core.SeedRefName: g.config.SeedConfig.SeedTemplate.Name}),
				},
				&operationsv1alpha1.Bastion{}: {
					Field: fields.SelectorFromSet(fields.Set{operations.BastionSeedName: g.config.SeedConfig.SeedTemplate.Name}),
				},
			}

			return kubernetes.AggregatorCacheFunc(
				kubernetes.NewRuntimeCache,
				map[client.Object]cache.NewCacheFunc{
					// Gardenlet should watch secrets only in the seed namespace of the seed it is responsible for. We don't use
					// any selector mechanism here since we want to still fall back to reading secrets with the API reader
					// (i.e., not from cache) in case the respective secret is not found in the cache.
					&corev1.Secret{}: cache.MultiNamespacedCacheBuilder([]string{gardenerutils.ComputeGardenNamespace(g.kubeconfigBootstrapResult.SeedName)}),
					// Gardenlet does not have the required RBAC permissions for listing/watching the following resources on cluster level.
					// Hence, we need to watch them individually with the help of a SingleObject cache.
					&corev1.ConfigMap{}:                         kubernetes.SingleObjectCacheFunc(log),
					&corev1.Namespace{}:                         kubernetes.SingleObjectCacheFunc(log),
					&coordinationv1.Lease{}:                     kubernetes.SingleObjectCacheFunc(log),
					&certificatesv1.CertificateSigningRequest{}: kubernetes.SingleObjectCacheFunc(log),
					&gardencorev1beta1.CloudProfile{}:           kubernetes.SingleObjectCacheFunc(log),
					&gardencorev1beta1.ControllerDeployment{}:   kubernetes.SingleObjectCacheFunc(log),
					&gardencorev1beta1.ExposureClass{}:          kubernetes.SingleObjectCacheFunc(log),
					&gardencorev1beta1.Project{}:                kubernetes.SingleObjectCacheFunc(log),
					&gardencorev1beta1.SecretBinding{}:          kubernetes.SingleObjectCacheFunc(log),
					&gardencorev1beta1.ShootState{}:             kubernetes.SingleObjectCacheFunc(log),
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

	log.Info("Updating last operation status of processing Shoots to 'Aborted'")
	if err := g.updateProcessingShootStatusToAborted(ctx, gardenCluster.GetClient()); err != nil {
		return err
	}

	// Migrate all relevant services in shoot control planes once, so that we don't have to wait for their reconciliation
	// and can ensure the required policies are created.
	// TODO(timuthy, rfranzke): To be removed in a future release.
	log.Info("Migrating all relevant shoot control plane services to create required network policies")
	if err := g.migrateAllShootServicesForNetworkPolicies(ctx, log); err != nil {
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

	log.Info("Adding runnables now that bootstrapping is finished")
	runnables := []manager.Runnable{
		g.healthManager,
		shootClientMap,
	}

	if g.config.GardenClientConnection.KubeconfigSecret != nil {
		certificateManager, err := certificate.NewCertificateManager(log, gardenCluster, g.mgr.GetClient(), g.config)
		if err != nil {
			return fmt.Errorf("failed to create a new certificate manager: %w", err)
		}

		runnables = append(runnables, manager.RunnableFunc(func(ctx context.Context) error {
			return certificateManager.ScheduleCertificateRotation(ctx, g.cancel, g.mgr.GetEventRecorderFor("certificate-manager"))
		}))
	}

	if err := controllerutils.AddAllRunnables(g.mgr, runnables...); err != nil {
		return err
	}

	log.Info("Adding controllers to manager")
	if err := controller.AddToManager(
		ctx,
		g.mgr,
		gardenCluster,
		g.mgr,
		shootClientMap,
		g.config,
		g.healthManager,
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
	cfg, err := gardenlethelper.ConvertGardenletConfigurationExternal(g.config)
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
		if err := gardenClient.Get(ctx, kubernetesutils.Key(gardenerutils.ComputeGardenNamespace(g.config.SeedConfig.Name)), &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

func (g *garden) migrateAllShootServicesForNetworkPolicies(ctx context.Context, log logr.Logger) error {
	var (
		taskFns                  []flow.TaskFn
		kubeApiServerServiceList = &corev1.ServiceList{}
	)

	// Kube-Apiserver services
	if err := g.mgr.GetClient().List(ctx, kubeApiServerServiceList, client.MatchingLabels{v1beta1constants.LabelApp: v1beta1constants.LabelKubernetes, v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer}); err != nil {
		return err
	}

	taskFns = append(taskFns, migrationTasksForServices(g.mgr.GetClient(), kubeApiServerServiceList.Items, kubeapiserverconstants.Port)...)

	// VPN-Seed-Server services
	for _, serviceName := range []string{vpnseedserver.ServiceName, vpnseedserver.ServiceName + "-0", vpnseedserver.ServiceName + "-1"} {
		serviceList := &corev1.ServiceList{}
		// Use APIReader here because an index on `metadata.name` is not available in the runtime client.
		if err := g.mgr.GetAPIReader().List(ctx, serviceList, client.MatchingFieldsSelector{
			Selector: fields.OneTermEqualSelector(metav1.ObjectNameField, serviceName),
		}); err != nil {
			return err
		}

		taskFns = append(taskFns, migrationTasksForServices(g.mgr.GetClient(), serviceList.Items, vpnseedserver.MetricsPort)...)
	}

	return flow.Parallel(taskFns...)(ctx)
}

func migrationTasksForServices(cl client.Client, services []corev1.Service, port int) []flow.TaskFn {
	var taskFns []flow.TaskFn

	for _, svc := range services {
		service := svc

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(service.DeepCopy())

			metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
			utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(&service,
				metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
				metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: v1beta1constants.LabelExposureClassHandlerName, Operator: metav1.LabelSelectorOpExists}}}))
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(&service, networkingv1.NetworkPolicyPort{Port: utils.IntStrPtrFromInt(port), Protocol: utils.ProtocolPtr(corev1.ProtocolTCP)}))

			return cl.Patch(ctx, &service, patch)
		})
	}

	return taskFns
}

func (g *garden) updateProcessingShootStatusToAborted(ctx context.Context, gardenClient client.Client) error {
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, s := range shootList.Items {
		shoot := s

		if specSeedName, statusSeedName := gardenerutils.GetShootSeedNames(&shoot); gardenerutils.GetResponsibleSeedName(specSeedName, statusSeedName) != g.config.SeedConfig.Name {
			continue
		}

		// Check if the status indicates that an operation is processing and mark it as "aborted".
		if shoot.Status.LastOperation == nil || shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateProcessing {
			continue
		}

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
			if err := gardenClient.Status().Patch(ctx, &shoot, patch); err != nil {
				return fmt.Errorf("failed to set status to 'Aborted' for shoot %q: %w", client.ObjectKeyFromObject(&shoot), err)
			}

			return nil
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

func addAllFieldIndexes(ctx context.Context, i client.FieldIndexer) error {
	for _, fn := range []func(context.Context, client.FieldIndexer) error{
		// core API group
		indexer.AddShootSeedName,
		indexer.AddShootStatusSeedName,
		indexer.AddBackupBucketSeedName,
		indexer.AddBackupEntrySeedName,
		indexer.AddBackupEntryBucketName,
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
