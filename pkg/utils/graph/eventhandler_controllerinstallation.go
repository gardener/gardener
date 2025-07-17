// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
		AddFunc: func(obj any) {
			controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return
			}
			g.handleControllerInstallationCreateOrUpdate(controllerInstallation)
		},

		UpdateFunc: func(oldObj, newObj any) {
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

		DeleteFunc: func(obj any) {
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
