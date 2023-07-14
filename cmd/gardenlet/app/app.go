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
	"os"
	goruntime "runtime"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
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
	controllerconfigv1alpha1 "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/cmd/gardenlet/app/bootstrappers"
	"github.com/gardener/gardener/pkg/api/indexer"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	"github.com/gardener/gardener/pkg/gardenlet/controller"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
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
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

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
					Field: fields.SelectorFromSet(fields.Set{gardencore.SeedRefName: g.config.SeedConfig.SeedTemplate.Name}),
				},
				&operationsv1alpha1.Bastion{}: {
					Field: fields.SelectorFromSet(fields.Set{operations.BastionSeedName: g.config.SeedConfig.SeedTemplate.Name}),
				},
			}

			return kubernetes.AggregatorCacheFunc(
				kubernetes.NewRuntimeCache,
				map[client.Object]cache.NewCacheFunc{
					// Gardenlet should watch secrets only in the seed namespace of the seed it is responsible for. We
					// don't use any selector mechanism here since we want to still fall back to reading secrets with
					// the API reader (i.e., not from cache) in case the respective secret is not found in the cache.
					&corev1.Secret{}:         cache.MultiNamespacedCacheBuilder([]string{gardenerutils.ComputeGardenNamespace(g.kubeconfigBootstrapResult.SeedName)}),
					&corev1.ServiceAccount{}: cache.MultiNamespacedCacheBuilder([]string{gardenerutils.ComputeGardenNamespace(g.kubeconfigBootstrapResult.SeedName)}),
					// Gardenlet does not have the required RBAC permissions for listing/watching the following
					// resources on cluster level. Hence, we need to watch them individually with the help of a
					// SingleObject cache.
					&corev1.ConfigMap{}:                         kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &corev1.ConfigMap{}),
					&corev1.Namespace{}:                         kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &corev1.Namespace{}),
					&coordinationv1.Lease{}:                     kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &coordinationv1.Lease{}),
					&certificatesv1.CertificateSigningRequest{}: kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &certificatesv1.CertificateSigningRequest{}),
					&gardencorev1beta1.CloudProfile{}:           kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.CloudProfile{}),
					&gardencorev1beta1.ControllerDeployment{}:   kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.ControllerDeployment{}),
					&gardencorev1beta1.ExposureClass{}:          kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.ExposureClass{}),
					&gardencorev1beta1.InternalSecret{}:         kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.InternalSecret{}),
					&gardencorev1beta1.Project{}:                kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.Project{}),
					&gardencorev1beta1.SecretBinding{}:          kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.SecretBinding{}),
					&gardencorev1beta1.ShootState{}:             kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.ShootState{}),
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

	// TODO(rfranzke): Remove this code after v1.74 has been released.
	{
		log.Info("Removing legacy ShootState controller finalizer from persistable secrets in seed cluster")
		if err := removeLegacyShootStateControllerFinalizerFromSecrets(ctx, g.mgr.GetClient()); err != nil {
			return err
		}
		if err := g.cleanupStaleShootStates(ctx, gardenCluster.GetClient()); err != nil {
			return err
		}
	}

	// TODO(shafeeqes): Remove this code in v1.77
	{
		log.Info("Cleaning up stale 'addons' ManagedResource from shoot namespaces in the seed cluster")
		if err := g.cleanupStaleAddonsMR(ctx, gardenCluster.GetClient(), g.mgr.GetClient()); err != nil {
			return err
		}
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
		g.cancel,
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
		seed.Annotations = utils.MergeStringMaps(seed.Annotations, g.config.SeedConfig.Annotations)
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

func removeLegacyShootStateControllerFinalizerFromSecrets(ctx context.Context, seedClient client.Client) error {
	secretList := &metav1.PartialObjectMetadataList{}
	secretList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
	if err := seedClient.List(ctx, secretList, client.MatchingLabels{
		secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyPersist:   secretsmanager.LabelValueTrue,
	}); err != nil {
		return fmt.Errorf("failed listing all secrets that must be persisted: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, s := range secretList.Items {
		secret := s

		taskFns = append(taskFns, func(ctx context.Context) error {
			if err := controllerutils.RemoveFinalizers(ctx, seedClient, &secret, "gardenlet.gardener.cloud/secret-controller"); err != nil {
				return fmt.Errorf("failed to remove legacy ShootState controller finalizer from secret %q: %w", client.ObjectKeyFromObject(&secret), err)
			}
			return nil
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

func (g *garden) cleanupStaleShootStates(ctx context.Context, gardenClient client.Client) error {
	if err := gardenClient.Get(ctx, client.ObjectKey{Name: g.config.SeedConfig.Name, Namespace: v1beta1constants.GardenNamespace}, &seedmanagementv1alpha1.ManagedSeed{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether gardenlet is responsible for a managed seed: %w", err)
		}
		return nil
	}

	g.mgr.GetLogger().Info("Removing stale ShootState resources from garden cluster since I'm responsible for a managed seed (GEP-22)")

	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList, client.MatchingFields{gardencore.ShootSeedName: g.config.SeedConfig.Name}); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, s := range shootList.Items {
		shoot := s

		// If status.seedName is different than seed name gardenlet is responsible for, then a migration takes place.
		// In this case, we don't want to delete the shoot state. It will be deleted eventually after successful
		// restoration by the shoot controller itself.
		if shoot.Status.SeedName != nil && *shoot.Status.SeedName != g.config.SeedConfig.Name {
			continue
		}

		// We don't want to delete the shoot state when the last operation type is 'Restore' (it might not be completed
		// yet). It will be deleted eventually after successful restoration by the shoot controller itself.
		if v1beta1helper.ShootHasOperationType(shoot.Status.LastOperation, gardencorev1beta1.LastOperationTypeRestore) {
			continue
		}

		taskFns = append(taskFns, func(ctx context.Context) error {
			return shootstate.Delete(ctx, gardenClient, &shoot)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

func (g *garden) cleanupStaleAddonsMR(ctx context.Context, gardenClient, seedClient client.Client) error {
	shootNamespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, shootNamespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, ns := range shootNamespaceList.Items {
		namespace := ns

		taskFns = append(taskFns, func(ctx context.Context) error {
			var (
				addonsMR = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addons",
						Namespace: namespace.Name,
					},
				}
				resourceManagerDeployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
						Namespace: namespace.Name,
					},
				}
			)

			if err := seedClient.Get(ctx, client.ObjectKeyFromObject(addonsMR), addonsMR); err != nil {
				// If the MR is already gone, then nothing to do here
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			if err := seedClient.Get(ctx, client.ObjectKeyFromObject(resourceManagerDeployment), resourceManagerDeployment); err != nil {
				return err
			}

			replicas := pointer.Int32Deref(resourceManagerDeployment.Spec.Replicas, 0)
			if replicas > 0 {
				patch := client.MergeFrom(resourceManagerDeployment.DeepCopy())
				resourceManagerDeployment.Spec.Replicas = pointer.Int32(0)
				if err := seedClient.Patch(ctx, resourceManagerDeployment, patch); err != nil {
					return fmt.Errorf("failed to scale gardener-resource-manager deployment to zero %q: %w", client.ObjectKeyFromObject(resourceManagerDeployment), err)
				}
			}

			patch := client.MergeFrom(addonsMR.DeepCopy())
			addonsMR.Finalizers = nil
			if err := seedClient.Patch(ctx, addonsMR, patch); err != nil {
				return fmt.Errorf("failed to patch ManagedResource %q: %w", client.ObjectKeyFromObject(addonsMR), err)
			}

			if err := managedresources.DeleteForShoot(ctx, seedClient, namespace.Name, "addons"); err != nil {
				return err
			}

			if err := managedresources.WaitUntilDeleted(ctx, seedClient, namespace.Name, "addons"); err != nil {
				return err
			}

			if replicas > 0 {
				patch := client.MergeFrom(resourceManagerDeployment.DeepCopy())
				resourceManagerDeployment.Spec.Replicas = &replicas
				if err := seedClient.Patch(ctx, resourceManagerDeployment, patch); err != nil {
					return fmt.Errorf("failed to scale gardener-resource-manager deployment back from zero %q: %w", client.ObjectKeyFromObject(resourceManagerDeployment), err)
				}
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
