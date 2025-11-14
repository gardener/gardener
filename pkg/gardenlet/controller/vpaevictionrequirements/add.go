// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpaevictionrequirements

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controller/vpaevictionrequirements"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

// SeedIsGardenCheckInterval is the interval how often it should be checked whether the seed cluster has been registered
// as garden cluster.
var SeedIsGardenCheckInterval = time.Minute

// AddToManager adds the controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	gardenletCancel context.CancelFunc,
	cfg gardenletconfigv1alpha1.VPAEvictionRequirementsControllerConfiguration,
	seedCluster cluster.Cluster,
) error {
	seedIsGarden, err := gardenletutils.SeedIsGarden(ctx, seedCluster.GetAPIReader())
	if err != nil {
		return fmt.Errorf("failed checking whether the seed is the garden cluster: %w", err)
	}
	if seedIsGarden {
		// When the seed is the garden cluster, the gardener-operator runs this controller.
		return nil
	}

	if !gardenletutils.IsResponsibleForSelfHostedShoot() {
		seedIsSelfHostedShoot, err := gardenletutils.SeedIsSelfHostedShoot(ctx, seedCluster.GetAPIReader())
		if err != nil {
			return fmt.Errorf("failed checking whether the seed is a self-hosted shoot cluster: %w", err)
		}
		if seedIsSelfHostedShoot {
			// When the seed is a self-hosted shoot cluster, the gardenlet responsible for the self-hosted shoot in the
			// kube-system namespace runs this controller.
			return nil
		}
	}

	if err := (&vpaevictionrequirements.Reconciler{
		ConcurrentSyncs: cfg.ConcurrentSyncs,
	}).AddToManager(mgr, seedCluster); err != nil {
		return err
	}

	// At this point, the seed is not the garden cluster. However, this could change during the runtime of gardenlet.
	// If so, gardener-operator will take over responsibility of the VPA eviction requirements and will run this
	// controller.
	// Similarly, the seed is not a self-hosted shoot cluster (at least not detectable right now, maybe because it was
	// not yet connected to Gardener, i.e., a gardenlet deployment in the kube-system namespace does not exist, but this
	// could change during the runtime of gardenlet). If so, the gardenlet in the kube-system namespace will take over
	// responsibility of the VPA eviction requirements and will run this controller.
	// Since there is no way to stop a controller after it started, we cancel the manager context in such cases. This
	// way, gardenlet will restart and not add the controller again.
	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		wait.Until(func() {
			seedIsGarden, err = gardenletutils.SeedIsGarden(ctx, seedCluster.GetClient())
			if err != nil {
				mgr.GetLogger().Error(err, "Failed checking whether the seed cluster is the garden cluster")
				return
			}
			if seedIsGarden {
				mgr.GetLogger().Info("Terminating gardenlet since seed cluster has been registered as garden cluster. " +
					"This effectively stops the VPAEvictionRequirements controller (gardener-operator takes over now).")
				gardenletCancel()
				return
			}

			if !gardenletutils.IsResponsibleForSelfHostedShoot() {
				seedIsSelfHostedShoot, err := gardenletutils.SeedIsSelfHostedShoot(ctx, seedCluster.GetAPIReader())
				if err != nil {
					mgr.GetLogger().Error(err, "Failed checking whether the seed cluster is a self-hosted shoot cluster")
					return
				}
				if seedIsSelfHostedShoot {
					mgr.GetLogger().Info("Terminating gardenlet since seed cluster has been detect to be a self-hosted " +
						"shoot cluster. This effectively stops the VPAEvictionRequirements controller (gardenlet in the " +
						"kube-system namespace takes over now).")
					gardenletCancel()
					return
				}
			}
		}, SeedIsGardenCheckInterval, ctx.Done())
		return nil
	}))
}
