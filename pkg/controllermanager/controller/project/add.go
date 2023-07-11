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

package project

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/activity"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/project"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/stale"
)

// AddToManager adds all Project controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg config.ControllerManagerConfiguration) error {
	if err := (&activity.Reconciler{
		Config: *cfg.Controllers.Project,
	}).AddToManager(ctx, mgr); err != nil {
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
