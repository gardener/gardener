// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package graph

import (
	"context"
	"time"

	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func (g *graph) setupControllerInstallationWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return
			}
			g.handleControllerInstallationCreateOrUpdate(controllerInstallation)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldControllerInstallation, ok := oldObj.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return
			}

			newControllerInstallation, ok := newObj.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return
			}

			var oldDeploymentRef string
			if oldControllerInstallation.Spec.DeploymentRef != nil {
				oldDeploymentRef = oldControllerInstallation.Spec.DeploymentRef.Name
			}
			var newDeploymentRef string
			if newControllerInstallation.Spec.DeploymentRef != nil {
				newDeploymentRef = newControllerInstallation.Spec.DeploymentRef.Name
			}

			if oldControllerInstallation.Spec.SeedRef.Name != newControllerInstallation.Spec.SeedRef.Name ||
				oldControllerInstallation.Spec.RegistrationRef.Name != newControllerInstallation.Spec.RegistrationRef.Name ||
				oldDeploymentRef != newDeploymentRef {
				g.handleControllerInstallationCreateOrUpdate(newControllerInstallation)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return
			}
			g.handleControllerInstallationDelete(controllerInstallation)
		},
	})
	return err
}

func (g *graph) handleControllerInstallationCreateOrUpdate(controllerInstallation *gardencorev1beta1.ControllerInstallation) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ControllerInstallation", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeControllerInstallation, "", controllerInstallation.Name)

	var (
		controllerInstallationVertex = g.getOrCreateVertex(VertexTypeControllerInstallation, "", controllerInstallation.Name)
		seedVertex                   = g.getOrCreateVertex(VertexTypeSeed, "", controllerInstallation.Spec.SeedRef.Name)
		controllerRegistrationVertex = g.getOrCreateVertex(VertexTypeControllerRegistration, "", controllerInstallation.Spec.RegistrationRef.Name)
	)

	g.addEdge(controllerRegistrationVertex, controllerInstallationVertex)
	g.addEdge(controllerInstallationVertex, seedVertex)

	if controllerInstallation.Spec.DeploymentRef != nil {
		controllerDeploymentVertex := g.getOrCreateVertex(VertexTypeControllerDeployment, "", controllerInstallation.Spec.DeploymentRef.Name)
		g.addEdge(controllerDeploymentVertex, controllerInstallationVertex)
	}
}

func (g *graph) handleControllerInstallationDelete(controllerInstallation *gardencorev1beta1.ControllerInstallation) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ControllerInstallation", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeControllerInstallation, "", controllerInstallation.Name)
}
