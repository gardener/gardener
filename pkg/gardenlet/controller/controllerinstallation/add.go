// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/required"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
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
	seedIsSelfHostedShoot bool,
	selfHostedShoot *gardencorev1beta1.Shoot,
	seedName string,
	gardenNamespace string,
) error {
	if gardenletutils.IsResponsibleForSelfHostedShoot() || !seedIsSelfHostedShoot {
		if err := (&care.Reconciler{
			Config:                   *cfg.Controllers.ControllerInstallationCare,
			ManagedResourceNamespace: gardenNamespace,
		}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
			return fmt.Errorf("failed adding care reconciler: %w", err)
		}

		var selfHostedShootMeta *types.NamespacedName
		if selfHostedShoot != nil {
			selfHostedShootMeta = &types.NamespacedName{Name: selfHostedShoot.Name, Namespace: selfHostedShoot.Namespace}
		}

		if err := (&controllerinstallation.Reconciler{
			SeedClientSet:         seedClientSet,
			Config:                cfg,
			Identity:              identity,
			GardenClusterIdentity: gardenClusterIdentity,
			GardenNamespace:       gardenNamespace,
			SelfHostedShootMeta:   selfHostedShootMeta,
		}).AddToManager(ctx, mgr, gardenCluster); err != nil {
			return fmt.Errorf("failed adding main reconciler: %w", err)
		}

		if err := (&required.Reconciler{
			Config:              *cfg.Controllers.ControllerInstallationRequired,
			SeedName:            seedName,
			SelfHostedShootMeta: selfHostedShootMeta,
		}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
			return fmt.Errorf("failed adding required reconciler: %w", err)
		}
	}

	return nil
}
