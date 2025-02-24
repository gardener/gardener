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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

func run(ctx context.Context, log logr.Logger, cfg *config.ControllerManagerConfiguration) error {
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

	restConfig, err := kubernetes.RESTConfigFromInternalClientConnectionConfiguration(&cfg.GardenClientConnection, nil, kubernetes.AuthTokenFile)
	if err != nil {
		return err
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && cfg.Debugging.EnableProfiling {
		extraHandlers = routes.ProfilingHandlers
		if cfg.Debugging.EnableContentionProfiling {
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

		LeaderElection:                cfg.LeaderElection.LeaderElect,
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

	// TODO(rfranzke): Remove this after Gardener v1.114 has been released and add code that cleans up all legacy
	//  `seed.gardener.cloud/<name>=true` labels from these objects.
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		var fns []flow.TaskFn

		prepareEmptyPatchTasks := func(list client.ObjectList, seedNamesFromObject func(obj client.Object) ([]*string, error)) error {
			if err := mgr.GetClient().List(ctx, list); err != nil {
				return fmt.Errorf("failed listing objects: %w", err)
			}

			return meta.EachListItem(list, func(o runtime.Object) error {
				fns = append(fns, func(ctx context.Context) error {
					obj := o.(client.Object)

					if slices.ContainsFunc(maps.Keys(obj.GetLabels()), func(s string) bool {
						return strings.HasPrefix(s, v1beta1constants.LabelPrefixSeedName)
					}) {
						return nil
					}

					gvk, err := apiutil.GVKForObject(obj, mgr.GetScheme())
					if err != nil {
						return fmt.Errorf("could not get GroupVersionKind from object %v: %w", obj, err)
					}

					mgr.GetLogger().Info("Adding new seed name labels", "gvk", gvk, "objectKey", client.ObjectKeyFromObject(obj))

					seedNames, err := seedNamesFromObject(obj)
					if err != nil {
						return err
					}

					patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
					gardenerutils.MaintainSeedNameLabels(obj, seedNames...)
					return mgr.GetClient().Patch(ctx, obj, patch)
				})
				return nil
			})
		}

		if err := prepareEmptyPatchTasks(&gardencorev1beta1.BackupEntryList{}, func(obj client.Object) ([]*string, error) {
			backupEntry := obj.(*gardencorev1beta1.BackupEntry)
			return []*string{backupEntry.Spec.SeedName, backupEntry.Status.SeedName}, nil
		}); err != nil {
			return fmt.Errorf("failed computing tasks for backup entries: %w", err)
		}

		if err := prepareEmptyPatchTasks(&gardencorev1beta1.ShootList{}, func(obj client.Object) ([]*string, error) {
			shoot := obj.(*gardencorev1beta1.Shoot)
			return []*string{shoot.Spec.SeedName, shoot.Status.SeedName}, nil
		}); err != nil {
			return fmt.Errorf("failed computing tasks for shoots: %w", err)
		}

		return flow.Parallel(fns...)(ctx)
	})); err != nil {
		return fmt.Errorf("failed adding seed name label migration runnable to manager: %w", err)
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
