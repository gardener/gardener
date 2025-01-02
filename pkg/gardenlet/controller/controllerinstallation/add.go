// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/required"
)

// AddToManager adds all ControllerInstallation controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	cfg gardenletconfigv1alpha1.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
) error {
	if err := (&care.Reconciler{
		Config: *cfg.Controllers.ControllerInstallationCare,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
	}

	if err := (&controllerinstallation.Reconciler{
		SeedClientSet:         seedClientSet,
		Config:                cfg,
		Identity:              identity,
		GardenClusterIdentity: gardenClusterIdentity,
	}).AddToManager(ctx, mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding main reconciler: %w", err)
	}

	if err := (&required.Reconciler{
		Config:   *cfg.Controllers.ControllerInstallationRequired,
		SeedName: cfg.SeedConfig.SeedTemplate.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding required reconciler: %w", err)
	}

	return nil
}
