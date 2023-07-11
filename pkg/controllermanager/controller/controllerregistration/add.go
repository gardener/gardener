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

package controllerregistration

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerregistrationfinalizer"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seed"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seedfinalizer"
)

// AddToManager adds all ControllerRegistration controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg config.ControllerManagerConfiguration) error {
	if err := (&seed.Reconciler{
		Config: *cfg.Controllers.ControllerRegistration,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding Seed reconciler: %w", err)
	}

	if err := (&controllerregistrationfinalizer.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding ControllerRegistration finalizer reconciler: %w", err)
	}

	if err := (&seedfinalizer.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Seed finalizer reconciler: %w", err)
	}

	return nil
}
