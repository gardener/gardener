// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/clusterfinalizer"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation/seed"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation/shoot"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerregistrationfinalizer"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/extensionclusterrole"
)

// AddToManager adds all ControllerRegistration controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	if err := (&clusterfinalizer.Reconciler{
		NewTargetObjectFunc: func() client.Object {
			return &gardencorev1beta1.Seed{}
		},
		NewControllerInstallationSelector: func(obj client.Object) client.MatchingFields {
			return client.MatchingFields{core.SeedRefName: obj.GetName()}
		},
	}).AddToManager(mgr, clusterfinalizer.MapControllerInstallationToSeed, "seed"); err != nil {
		return fmt.Errorf("failed adding cluster finalizer reconciler for Seeds: %w", err)
	}

	if err := (&clusterfinalizer.Reconciler{
		NewTargetObjectFunc: func() client.Object {
			return &gardencorev1beta1.Shoot{}
		},
		NewControllerInstallationSelector: func(obj client.Object) client.MatchingFields {
			return client.MatchingFields{core.ShootRefName: obj.GetName(), core.ShootRefNamespace: obj.GetNamespace()}
		},
	}).AddToManager(mgr, clusterfinalizer.MapControllerInstallationToShoot, "shoot"); err != nil {
		return fmt.Errorf("failed adding cluster finalizer reconciler for Shoots: %w", err)
	}

	if err := seed.AddToManager(mgr, *cfg.Controllers.ControllerRegistration); err != nil {
		return fmt.Errorf("failed adding ControllerInstallation Seed reconciler: %w", err)
	}

	if err := shoot.AddToManager(mgr, *cfg.Controllers.ControllerRegistration); err != nil {
		return fmt.Errorf("failed adding ControllerInstallation Shoot reconciler: %w", err)
	}

	if err := (&controllerregistrationfinalizer.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding ControllerRegistration finalizer reconciler: %w", err)
	}

	if err := (&extensionclusterrole.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding extension ClusterRole reconciler: %w", err)
	}

	return nil
}
