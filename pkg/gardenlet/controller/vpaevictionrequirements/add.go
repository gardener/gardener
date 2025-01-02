// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
		return nil // When the seed is the garden cluster at the same time, the gardener-operator runs this controller.
	}

	if err := (&vpaevictionrequirements.Reconciler{
		ConcurrentSyncs: cfg.ConcurrentSyncs,
	}).AddToManager(mgr, seedCluster); err != nil {
		return err
	}

	// At this point, the seed is not the garden cluster at the same time. However, this could change during the runtime
	// of gardenlet. If so, gardener-operator will take over responsibility of the VPA eviction requirements and will run
	// this controller. Since there is no way to stop a controller after it started, we cancel the manager context in
	// case the seed is registered as garden during runtime. This way, gardenlet will restart and not add the controller
	// again.
	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		wait.Until(func() {
			seedIsGarden, err = gardenletutils.SeedIsGarden(ctx, seedCluster.GetClient())
			if err != nil {
				mgr.GetLogger().Error(err, "Failed checking whether the seed cluster is the garden cluster at the same time")
				return
			}
			if !seedIsGarden {
				return
			}

			mgr.GetLogger().Info("Terminating gardenlet since seed cluster has been registered as garden cluster. " +
				"This effectively stops the VPAEvictionRequirements controller (gardener-operator takes over now).")
			gardenletCancel()
		}, SeedIsGardenCheckInterval, ctx.Done())
		return nil
	}))
}
