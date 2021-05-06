// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
)

func (c *Controller) controllerDeploymentAdd(ctx context.Context, obj interface{}) {
	controllerDeployment, ok := obj.(*gardencorev1beta1.ControllerDeployment)
	if !ok {
		return
	}

	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := c.gardenClient.List(ctx, controllerRegistrationList); err != nil {
		logger.Logger.Errorf("error listing controllerregistrations: %+v", err)
		return
	}

	for _, controllerReg := range controllerRegistrationList.Items {
		deployment := controllerReg.Spec.Deployment
		if deployment == nil {
			continue
		}
		for _, ref := range deployment.DeploymentRefs {
			if ref.Name == controllerDeployment.Name {
				c.enqueueAllSeeds(ctx)
				return
			}
		}
	}
}

func (c *Controller) controllerDeploymentUpdate(ctx context.Context, _, newObj interface{}) {
	c.controllerDeploymentAdd(ctx, newObj)
}
