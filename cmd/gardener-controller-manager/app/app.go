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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/version/verflag"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/cmd/gardener-controller-manager/app/bootstrappers"
	"github.com/gardener/gardener/cmd/utils/initrun"
	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// Name is a const for the name of this component.
const Name = "gardener-controller-manager"

// NewCommand creates a new cobra.Command for running gardener-controller-manager.
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

func run(ctx context.Context, log logr.Logger, cfg *controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

	// This is like importing the automaxprocs package for its init func (it will in turn call maxprocs.Set).
	// Here we pass a custom logger, so that the result of the library gets logged to the same logger we use for the
	// component itself.
	if _, err := maxprocs.Set(maxprocs.Logger(func(s string, i ...any) {
		log.Info(fmt.Sprintf(s, i...)) //nolint:logcheck
	})); err != nil {
		log.Error(err, "Failed to set GOMAXPROCS")
	}

	log.Info("Getting rest config")
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.GardenClientConnection.Kubeconfig = kubeconfig
	}

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.GardenClientConnection, nil, kubernetes.AuthTokenFile)
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
	mgr, err := manager.New(restConfig, manager.Options{
		Logger:                  log,
		Scheme:                  kubernetes.GardenScheme,
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

	log.Info("Adding field indexes to informers")
	if err := addAllFieldIndexes(ctx, mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf("failed adding indexes: %w", err)
	}

	log.Info("Adding garden bootstrapper to manager")
	if err := mgr.Add(&bootstrappers.Bootstrapper{
		Log:        log.WithName("bootstrap"),
		Client:     mgr.GetClient(),
		RESTConfig: mgr.GetConfig(),
	}); err != nil {
		return fmt.Errorf("failed adding garden cluster bootstrapper to manager: %w", err)
	}

	log.Info("Adding controllers to manager")
	if err := controller.AddToManager(ctx, mgr, cfg); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	// TODO(rfranzke): Remove this runnable after Gardener v1.119 has been released.
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		var fns []flow.TaskFn

		seedList := &gardencorev1beta1.SeedList{}
		if err := mgr.GetClient().List(ctx, seedList); err != nil {
			return fmt.Errorf("failed listing seeds: %w", err)
		}

		seedNames := sets.New[string]()
		for _, seed := range seedList.Items {
			seedNames.Insert(seed.Name)
		}

		for _, list := range []client.ObjectList{
			&gardencorev1beta1.BackupEntryList{},
			&gardencorev1beta1.ShootList{},
			&gardencorev1beta1.SeedList{},
			&seedmanagementv1alpha1.ManagedSeedList{},
		} {
			if err := mgr.GetClient().List(ctx, list); err != nil {
				return fmt.Errorf("failed listing objects: %w", err)
			}

			if err := meta.EachListItem(list, func(o runtime.Object) error {
				fns = append(fns, func(ctx context.Context) error {
					obj := o.(client.Object)

					gvk, err := apiutil.GVKForObject(obj, mgr.GetScheme())
					if err != nil {
						return fmt.Errorf("could not get GroupVersionKind from object %v: %w", obj, err)
					}

					var patchNeeded bool

					patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
					labels := obj.GetLabels()
					for k, v := range labels {
						if strings.HasPrefix(k, "seed.gardener.cloud/") && v == "true" && seedNames.Has(strings.TrimPrefix(k, "seed.gardener.cloud/")) {
							delete(labels, k)
							patchNeeded = true
						}
					}

					if patchNeeded {
						mgr.GetLogger().Info("Removing legacy seed name labels", "gvk", gvk, "objectKey", client.ObjectKeyFromObject(obj))

						obj.SetLabels(labels)

						return mgr.GetClient().Patch(ctx, obj, patch)
					}

					return nil
				})
				return nil
			}); err != nil {
				return fmt.Errorf("failed preparing removal tasks for %T: %w", list, err)
			}
		}

		managedSeedList := &seedmanagementv1alpha1.ManagedSeedList{}
		if err := mgr.GetClient().List(ctx, managedSeedList); err != nil {
			return fmt.Errorf("failed listing managed seeds: %w", err)
		}
		for _, managedSeed := range managedSeedList.Items {
			fns = append(fns, func(ctx context.Context) error {
				obj := &managedSeed

				logger := mgr.GetLogger().WithValues("objectKey", client.ObjectKeyFromObject(obj))
				logger.Info("Check the seed name label for itself")
				label := v1beta1constants.LabelPrefixSeedName + obj.GetName()
				logger = logger.WithValues("label", label)
				if _, ok := obj.GetLabels()[label]; ok {
					logger.Info("Label exists, send an empty patch to the ManagedSeed so that the mutating webhook can remove the superfluous seed name label")
					emptyPatch := client.MergeFrom(obj)
					if err := mgr.GetClient().Patch(ctx, obj, emptyPatch); err != nil {
						return fmt.Errorf("failed to patch managed seed %s: %w", client.ObjectKeyFromObject(obj), err)
					}

					// assert the mutating webhook runs on the correct version and removed the label
					if _, ok := obj.GetLabels()[label]; ok {
						return fmt.Errorf("the label %s on the managed seed %s is still present, the mutating webhook is running in an older version", label, client.ObjectKeyFromObject(obj))
					}
				} else {
					logger.Info("The label on the managed seed is not present, nothing to do")
				}
				return nil
			})
		}
		return flow.Parallel(fns...)(ctx)
	})); err != nil {
		return fmt.Errorf("failed adding seed name label removal runnable to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func addAllFieldIndexes(ctx context.Context, i client.FieldIndexer) error {
	for _, fn := range []func(context.Context, client.FieldIndexer) error{
		// core API group
		indexer.AddProjectNamespace,
		indexer.AddShootSeedName,
		indexer.AddShootStatusSeedName,
		indexer.AddBackupBucketSeedName,
		indexer.AddBackupEntrySeedName,
		indexer.AddControllerInstallationSeedRefName,
		indexer.AddControllerInstallationRegistrationRefName,
		indexer.AddNamespacedCloudProfileParentRefName,
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
