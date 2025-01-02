// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/health"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/progressing"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// AddToManager adds all health controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, cfg config.ResourceManagerConfiguration) error {
	if err := (&health.Reconciler{
		Config:      cfg.Controllers.Health,
		ClassFilter: resourcemanagerpredicate.NewClassFilter(*cfg.Controllers.ResourceClass),
	}).AddToManager(mgr, sourceCluster, targetCluster, *cfg.Controllers.ClusterID); err != nil {
		return fmt.Errorf("failed adding health reconciler: %w", err)
	}

	if err := (&progressing.Reconciler{
		Config:      cfg.Controllers.Health,
		ClassFilter: resourcemanagerpredicate.NewClassFilter(*cfg.Controllers.ResourceClass),
	}).AddToManager(ctx, mgr, sourceCluster, targetCluster, *cfg.Controllers.ClusterID); err != nil {
		return fmt.Errorf("failed adding progressing reconciler: %w", err)
	}

	return nil
}
