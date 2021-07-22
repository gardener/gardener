// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package project

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "project-controller"

	activeProjectsQueue = "active"
	staleProjectsQueue  = "stale"
)

// AddToManager adds a new project controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	config *config.ProjectControllerConfiguration,
) error {
	logger := mgr.GetLogger()
	gardenClient := mgr.GetClient()

	reconciler := controllerutils.NewMultiplexReconciler(map[string]reconcile.Reconciler{
		activeProjectsQueue: NewProjectReconciler(logger, config, gardenClient, mgr.GetEventRecorderFor(ControllerName)),
		staleProjectsQueue:  NewProjectStaleReconciler(logger, config, gardenClient),
	})

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{}
	if err := c.Watch(&source.Kind{Type: roleBinding}, newRoleBindingEventHandler(ctx, gardenClient, logger)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", roleBinding, err)
	}

	project := &gardencorev1beta1.Project{}
	if err := c.Watch(&source.Kind{Type: project}, newProjectEventHandler()); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", project, err)
	}

	return nil
}
