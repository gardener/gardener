// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/predicate"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "shoot-scheduler"
)

// AddToManager adds a new scheduler controller to the given manager.
func AddToManager(
	mgr manager.Manager,
	config *config.ShootSchedulerConfiguration,
) error {
	ctrlOptions := controller.Options{
		Reconciler: &reconciler{
			config:       config,
			logger:       mgr.GetLogger().WithName(ControllerName),
			gardenClient: mgr.GetClient(),
			recorder:     mgr.GetEventRecorderFor(ControllerName),
		},
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}

	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := c.Watch(&source.Kind{Type: shoot}, &handler.EnqueueRequestForObject{},
		predicate.ShootIsUnassigned(),
		predicate.Not(predicate.IsDeleting()),
	); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %v", shoot, err)
	}

	return nil
}
