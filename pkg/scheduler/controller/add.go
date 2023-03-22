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

package controller

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	"github.com/gardener/gardener/pkg/scheduler/controller/shoot"
)

// AddToManager adds all scheduler controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg *config.SchedulerConfiguration) error {
	if err := (&shoot.Reconciler{
		Config: cfg.Schedulers.Shoot,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Shoot controller: %w", err)
	}

	return nil
}
