// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/scheduler/controller/shoot"
)

// AddToManager adds all scheduler controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg *schedulerconfigv1alpha1.SchedulerConfiguration) error {
	if err := (&shoot.Reconciler{
		Config: cfg.Schedulers.Shoot,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Shoot controller: %w", err)
	}

	return nil
}
