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

package shoot

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
)

// Controller controls Shoots.
type Controller struct {
	gardenClient client.Client
	log          logr.Logger
	config       *config.GardenletConfiguration
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewShootController(
	log logr.Logger,
	gardenCluster cluster.Cluster,
	config *config.GardenletConfiguration,
) (
	*Controller,
	error,
) {
	shootController := &Controller{
		log:          log,
		gardenClient: gardenCluster.GetClient(),
		config:       config,
	}

	return shootController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context) {
	// Update Shoots before starting the workers.
	shootFilterFunc := controllerutils.ShootFilterFunc(confighelper.SeedNameFromSeedConfig(c.config.SeedConfig))
	shoots := &gardencorev1beta1.ShootList{}
	if err := c.gardenClient.List(ctx, shoots); err != nil {
		c.log.Error(err, "Failed to fetch shoots resources")
		return
	}

	for _, shoot := range shoots.Items {
		if !shootFilterFunc(&shoot) {
			continue
		}

		// Check if the status indicates that an operation is processing and mark it as "aborted".
		if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateProcessing {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
			if err := c.gardenClient.Status().Patch(ctx, &shoot, patch); err != nil {
				panic(fmt.Sprintf("Failed to update shoot status [%s]: %v ", client.ObjectKeyFromObject(&shoot).String(), err.Error()))
			}
		}
	}
}
