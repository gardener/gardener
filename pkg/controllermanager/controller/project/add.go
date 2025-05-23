// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/activity"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/project"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/stale"
)

// AddToManager adds all Project controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	if err := (&activity.Reconciler{
		Config: *cfg.Controllers.Project,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding activity reconciler: %w", err)
	}

	if err := (&project.Reconciler{
		Config: *cfg.Controllers.Project,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding main reconciler: %w", err)
	}

	if err := (&stale.Reconciler{
		Config: *cfg.Controllers.Project,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding stale reconciler: %w", err)
	}

	return nil
}
