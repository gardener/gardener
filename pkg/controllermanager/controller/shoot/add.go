// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/conditions"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/hibernation"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/maintenance"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/quota"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/reference"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/retry"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/statuslabel"
)

// AddToManager adds all Shoot controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg config.ControllerManagerConfiguration) error {
	if err := (&conditions.Reconciler{
		Config: *cfg.Controllers.ShootConditions,
	}).AddToManager(ctx, mgr); err != nil {
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

	return nil
}
