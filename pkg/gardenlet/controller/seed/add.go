// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/lease"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/seed"
	"github.com/gardener/gardener/pkg/healthz"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all Seed controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	cfg gardenletconfigv1alpha1.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	healthManager healthz.Manager,
) error {
	var (
		componentImageVectors imagevectorutils.ComponentImageVectors
		err                   error
	)

	if path := os.Getenv(imagevectorutils.ComponentOverrideEnv); path != "" {
		componentImageVectors, err = imagevectorutils.ReadComponentOverwriteFile(path)
		if err != nil {
			return fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	if err := (&care.Reconciler{
		Config:   *cfg.Controllers.SeedCare,
		SeedName: cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
	}

	if err := (&lease.Reconciler{
		SeedRESTClient: seedClientSet.RESTClient(),
		Config:         *cfg.Controllers.Seed,
		HealthManager:  healthManager,
		SeedName:       cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding lease reconciler: %w", err)
	}

	if err := (&seed.Reconciler{
		SeedClientSet:         seedClientSet,
		Config:                cfg,
		Identity:              identity,
		ComponentImageVectors: componentImageVectors,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding main reconciler: %w", err)
	}

	return nil
}
