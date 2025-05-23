// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerregistrationfinalizer"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/extensionclusterrole"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seed"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seedfinalizer"
)

// AddToManager adds all ControllerRegistration controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	if err := (&seed.Reconciler{
		Config: *cfg.Controllers.ControllerRegistration,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Seed reconciler: %w", err)
	}

	if err := (&controllerregistrationfinalizer.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding ControllerRegistration finalizer reconciler: %w", err)
	}

	if err := (&extensionclusterrole.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding extension ClusterRole reconciler: %w", err)
	}

	if err := (&seedfinalizer.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Seed finalizer reconciler: %w", err)
	}

	return nil
}
