// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/conditions"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/hibernation"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/maintenance"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/migration"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/quota"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/reference"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/retry"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/state/finalizer"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/statuslabel"
)

// AddToManager adds all Shoot controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	if err := (&conditions.Reconciler{
		Config: *cfg.Controllers.ShootConditions,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding conditions reconciler: %w", err)
	}

	if err := (&hibernation.Reconciler{
		Config: cfg.Controllers.ShootHibernation,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding hibernation reconciler: %w", err)
	}

	if err := (&maintenance.Reconciler{
		Config: cfg.Controllers.ShootMaintenance,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding maintenance reconciler: %w", err)
	}

	if err := (&quota.Reconciler{
		Config: *cfg.Controllers.ShootQuota,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding quota reconciler: %w", err)
	}

	if err := (&migration.Reconciler{
		Config: *cfg.Controllers.ShootMigration,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding migration reconciler: %w", err)
	}

	if err := reference.AddToManager(mgr, *cfg.Controllers.ShootReference); err != nil {
		return fmt.Errorf("failed adding reference reconciler: %w", err)
	}

	if err := (&retry.Reconciler{
		Config: *cfg.Controllers.ShootRetry,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding retry reconciler: %w", err)
	}

	if err := (&statuslabel.Reconciler{
		Config: *cfg.Controllers.ShootStatusLabel,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding statuslabel reconciler: %w", err)
	}

	if err := (&finalizer.Reconciler{
		Config: *cfg.Controllers.ShootState,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding statuslabel reconciler: %w", err)
	}

	return nil
}
