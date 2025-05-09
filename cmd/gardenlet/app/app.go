// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
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

	"github.com/gardener/gardener/cmd/utils/initrun"
	"github.com/gardener/gardener/pkg/api/indexer"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrappers"
	"github.com/gardener/gardener/pkg/gardenlet/controller"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

	return cmd
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, cfg *gardenletconfigv1alpha1.GardenletConfiguration) error {
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

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && ptr.Deref(cfg.Debugging.EnableProfiling, false) {
		extraHandlers = routes.ProfilingHandlers
		if ptr.Deref(cfg.Debugging.EnableContentionProfiling, false) {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(seedRESTConfig, manager.Options{
		Logger:                  log,
		Scheme:                  kubernetes.SeedScheme,
		GracefulShutdownTimeout: ptr.To(5 * time.Second),

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

		MapperProvider: apiutil.NewDynamicRESTMapper,
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.Event{},
					&eventsv1.Event{},
				},
			},
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
	if err := mgr.AddHealthzCheck("seed-informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, mgr.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("seed-informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
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
	config                    *gardenletconfigv1alpha1.GardenletConfiguration
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

		opts.Client.Cache = &client.CacheOptions{
			DisableFor: []client.Object{
				&corev1.Event{},
				&eventsv1.Event{},
			},
		}

		seedNamespace := gardenerutils.ComputeGardenNamespace(g.config.SeedConfig.Name)

		opts.NewCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			// gardenlet should watch only objects which are related to the seed it is responsible for.
			opts.ByObject = map[client.Object]cache.ByObject{
				&gardencorev1beta1.ControllerInstallation{}: {
					Field: fields.SelectorFromSet(fields.Set{gardencore.SeedRefName: g.config.SeedConfig.Name}),
				},
				&gardencorev1beta1.Seed{}: {
					Label: labels.SelectorFromSet(labels.Set{v1beta1constants.LabelPrefixSeedName + g.config.SeedConfig.Name: "true"}),
				},
				&gardencorev1beta1.Shoot{}: {
					Label: labels.SelectorFromSet(labels.Set{v1beta1constants.LabelPrefixSeedName + g.config.SeedConfig.Name: "true"}),
				},
				&operationsv1alpha1.Bastion{}: {
					Field: fields.SelectorFromSet(fields.Set{operations.BastionSeedName: g.config.SeedConfig.Name}),
				},
				// Gardenlet should watch secrets/serviceAccounts only in the seed namespace of the seed it is responsible for.
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{seedNamespace: {}},
				},
				&corev1.ServiceAccount{}: {
					Namespaces: map[string]cache.Config{seedNamespace: {}},
				},
				&seedmanagementv1alpha1.ManagedSeed{}: {
					Label: labels.SelectorFromSet(labels.Set{v1beta1constants.LabelPrefixSeedName + g.config.SeedConfig.Name: "true"}),
				},
				&seedmanagementv1alpha1.Gardenlet{}: {
					Field:      fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: g.config.SeedConfig.Name}),
					Namespaces: map[string]cache.Config{v1beta1constants.GardenNamespace: {}},
				},
			}

			return kubernetes.AggregatorCacheFunc(
				kubernetes.NewRuntimeCache,
				map[client.Object]cache.NewCacheFunc{
					// Gardenlet does not have the required RBAC permissions for listing/watching the following
					// resources on cluster level. Hence, we need to watch them individually with the help of a
					// SingleObject cache.
					&corev1.ConfigMap{}:                         kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &corev1.ConfigMap{}),
					&corev1.Namespace{}:                         kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &corev1.Namespace{}),
					&coordinationv1.Lease{}:                     kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &coordinationv1.Lease{}),
					&certificatesv1.CertificateSigningRequest{}: kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &certificatesv1.CertificateSigningRequest{}),
					&gardencorev1.ControllerDeployment{}:        kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1.ControllerDeployment{}),
					&gardencorev1beta1.CloudProfile{}:           kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.CloudProfile{}),
					&gardencorev1beta1.NamespacedCloudProfile{}: kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.NamespacedCloudProfile{}),
					&gardencorev1beta1.ExposureClass{}:          kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.ExposureClass{}),
					&gardencorev1beta1.InternalSecret{}:         kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.InternalSecret{}),
					&gardencorev1beta1.Project{}:                kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.Project{}),
					&gardencorev1beta1.SecretBinding{}:          kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.SecretBinding{}),
					&gardencorev1beta1.ShootState{}:             kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &gardencorev1beta1.ShootState{}),
					&securityv1alpha1.CredentialsBinding{}:      kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &securityv1alpha1.CredentialsBinding{}),
					&securityv1alpha1.WorkloadIdentity{}:        kubernetes.SingleObjectCacheFunc(log, kubernetes.GardenScheme, &securityv1alpha1.WorkloadIdentity{}),
				},
				kubernetes.GardenScheme,
			)(config, opts)
		}

		// The created multi-namespace cache does not fall back to an uncached reader in case the gardenlet tries to
		// read a secret from another namespace. There might be secrets in namespace other than the seed-specific
		// namespace (e.g., backup secret in the SeedSpec). Hence, let's use a fallback client which falls back to an
		// uncached reader in case it fails to read objects from the cache.
		opts.NewClient = func(config *rest.Config, options client.Options) (client.Client, error) {
			uncachedOptions := options
			uncachedOptions.Cache = nil
			uncachedClient, err := client.New(config, uncachedOptions)
			if err != nil {
				return nil, err
			}

			cachedClient, err := client.New(config, options)
			if err != nil {
				return nil, err
			}

			return &kubernetes.FallbackClient{
				Client: cachedClient,
				Reader: uncachedClient,
				KindToNamespaces: map[string]sets.Set[string]{
					"Secret":         sets.New(seedNamespace),
					"ServiceAccount": sets.New(seedNamespace),
				},
			}, nil
		}

		opts.MapperProvider = apiutil.NewDynamicRESTMapper
	})
	if err != nil {
		return fmt.Errorf("failed creating garden cluster object: %w", err)
	}

	log.Info("Cleaning bootstrap authentication data used to request a certificate if needed")
	if len(g.kubeconfigBootstrapResult.CSRName) > 0 && len(g.kubeconfigBootstrapResult.SeedName) > 0 {
		if err := bootstrap.DeleteBootstrapAuth(ctx, gardenCluster.GetAPIReader(), gardenCluster.GetClient(), g.kubeconfigBootstrapResult.CSRName); err != nil {
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

	waitForSyncCtx, waitForSyncCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitForSyncCancel()

	log.V(1).Info("Waiting for cache to be synced")
	if !gardenCluster.GetCache().WaitForCacheSync(waitForSyncCtx) {
		return fmt.Errorf("failed waiting for cache to be synced")
	}

	log.Info("Registering Seed object in garden cluster")
	if err := g.registerSeed(ctx, gardenCluster.GetClient()); err != nil {
		return err
	}

	log.Info("Create Gardenlet object in garden cluster for self-upgrades if necessary")
	if err := g.createSelfUpgradeConfig(ctx, log, gardenCluster.GetClient()); err != nil {
		return err
	}

	log.Info("Updating last operation status of processing Shoots to 'Aborted'")
	if err := g.updateProcessingShootStatusToAborted(ctx, gardenCluster.GetClient()); err != nil {
		return err
	}

	if err := g.runMigrations(ctx, log, gardenCluster.GetClient()); err != nil {
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

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, gardenClient, seed, func() error {
		seed.Annotations = utils.MergeStringMaps(seed.Annotations, g.config.SeedConfig.Annotations)
		seed.Labels = utils.MergeStringMaps(map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
		}, g.config.SeedConfig.Labels)

		seed.Spec = g.config.SeedConfig.Spec
		return nil
	}); err != nil {
		return fmt.Errorf("could not register seed %q: %w", seed.Name, err)
	}

	// Verify that the gardener-controller-manager has created the seed-specific namespace. Here we also accept
	// 'forbidden' errors since the SeedAuthorizer (if enabled) forbid the gardenlet to read the namespace in case the
	// gardener-controller-manager has not yet created it.
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return wait.PollUntilContextCancel(timeoutCtx, 500*time.Millisecond, false, func(context.Context) (done bool, err error) {
		if err := gardenClient.Get(ctx, client.ObjectKey{Name: gardenerutils.ComputeGardenNamespace(g.config.SeedConfig.Name)}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

var seedManagementDecoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(seedmanagementv1alpha1.AddToScheme(scheme))
	seedManagementDecoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

func (g *garden) createSelfUpgradeConfig(ctx context.Context, log logr.Logger, gardenClient client.Client) error {
	var (
		gardenlet = &seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Name: g.config.SeedConfig.Name}}
		configMap = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet-selfupgrade-config", Namespace: v1beta1constants.GardenNamespace}}
	)

	if err := g.mgr.GetClient().Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether ConfigMap %q with seedmanagement.gardener.cloud/v1alpha1.Gardenlet object for self-upgrades exists: %w", client.ObjectKeyFromObject(configMap), err)
		}

		log.Info("ConfigMap does not exist, hence, no need to create seedmanagement.gardener.cloud/v1alpha1.Gardenlet object for self-upgrades", "configMap", client.ObjectKeyFromObject(configMap))
		return nil
	}

	if err := runtime.DecodeInto(seedManagementDecoder, []byte(configMap.Data["gardenlet.yaml"]), gardenlet); err != nil {
		return fmt.Errorf("error decoding seedmanagement.gardener.cloud/v1alpha1.Gardenlet object from ConfigMap %s: %w", client.ObjectKeyFromObject(configMap), err)
	}

	if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether seedmanagement.gardener.cloud/v1alpha1.Gardenlet object with name %q exists: %w", gardenlet.Name, err)
		}

		log.Info("The seedmanagement.gardener.cloud/v1alpha1.Gardenlet object for self-upgrades does not exist in garden cluster yet, creating it")
		if err := gardenClient.Create(ctx, gardenlet); err != nil {
			return fmt.Errorf("failed creating seedmanagement.gardener.cloud/v1alpha1.Gardenlet object for self-upgrades: %w", err)
		}
		log.Info("Successfully created seedmanagement.gardener.cloud/v1alpha1.Gardenlet object for self-upgrades")
	} else {
		log.Info("The seedmanagement.gardener.cloud/v1alpha1.Gardenlet object for self-upgrades already exists, nothing to do")
	}

	return client.IgnoreNotFound(g.mgr.GetClient().Delete(ctx, configMap))
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
