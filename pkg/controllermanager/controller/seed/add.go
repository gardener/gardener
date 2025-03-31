// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/backupbucketscheck"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/extensionscheck"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/lifecycle"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/reference"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/secrets"
)

// AddToManager adds all Seed controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	if err := (&backupbucketscheck.Reconciler{
		Config: *cfg.Controllers.SeedBackupBucketsCheck,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding backupbuckets check reconciler: %w", err)
	}

	if err := (&extensionscheck.Reconciler{
		Config: *cfg.Controllers.SeedExtensionsCheck,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding extensions check reconciler: %w", err)
	}

	if err := (&lifecycle.Reconciler{
		Config: *cfg.Controllers.Seed,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding lifecycle reconciler: %w", err)
	}

	if err := (&secrets.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding secrets reconciler: %w", err)
	}

	if err := reference.AddToManager(mgr, v1beta1constants.GardenNamespace, *cfg.Controllers.SeedReference); err != nil {
		return fmt.Errorf("failed adding reference reconciler: %w", err)
	}

	return nil
}
