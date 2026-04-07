// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

func (g *graph) setupControllerDeploymentWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			name, secretRefs := extractControllerDeploymentInfo(obj)
			if len(secretRefs) > 0 {
				g.handleControllerDeploymentCreateOrUpdate(name, secretRefs)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			_, oldSecretRefs := extractControllerDeploymentInfo(oldObj)
			name, newSecretRefs := extractControllerDeploymentInfo(newObj)

			if !sets.New(oldSecretRefs...).Equal(sets.New(newSecretRefs...)) && len(newSecretRefs) > 0 {
				g.handleControllerDeploymentCreateOrUpdate(name, newSecretRefs)
			}
		},
		DeleteFunc: func(obj any) {
			name, secretRefs := extractControllerDeploymentInfo(obj)
			if len(secretRefs) > 0 {
				g.handleControllerDeploymentDelete(name, secretRefs)
			}
		},
	})
	return err
}

func extractControllerDeploymentInfo(obj any) (string, []string) {
	controllerDeployment, ok := obj.(*gardencorev1.ControllerDeployment)
	if !ok {
		return "", nil
	}

	if controllerDeployment.Helm == nil || controllerDeployment.Helm.OCIRepository == nil {
		return controllerDeployment.Name, nil
	}

	secretNames := make([]string, 0, 2)
	if controllerDeployment.Helm.OCIRepository.PullSecretRef != nil {
		secretNames = append(secretNames, controllerDeployment.Helm.OCIRepository.PullSecretRef.Name)
	}
	if controllerDeployment.Helm.OCIRepository.CABundleSecretRef != nil {
		secretNames = append(secretNames, controllerDeployment.Helm.OCIRepository.CABundleSecretRef.Name)
	}

	return controllerDeployment.Name, secretNames
}

func (g *graph) handleControllerDeploymentCreateOrUpdate(controllerDeploymentName string, secretNames []string) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ControllerDeployment", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()

	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeControllerDeployment, "", controllerDeploymentName)

	controllerDeploymentVertex := g.getOrCreateVertex(VertexTypeControllerDeployment, "", controllerDeploymentName)
	for _, secretName := range secretNames {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, v1beta1constants.GardenNamespace, secretName)
		g.addEdge(secretVertex, controllerDeploymentVertex)
	}
}

func (g *graph) handleControllerDeploymentDelete(controllerDeploymentName string, _ []string) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ControllerDeployment", "Delete").Observe(time.Since(start).Seconds())
	}()

	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeControllerDeployment, "", controllerDeploymentName)
}
