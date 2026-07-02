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
			name, secretRefs, configMapRefs := extractControllerDeploymentInfo(obj)
			if len(secretRefs) > 0 || len(configMapRefs) > 0 {
				g.handleControllerDeploymentCreateOrUpdate(name, secretRefs, configMapRefs)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			_, oldSecretRefs, oldConfigMapRefs := extractControllerDeploymentInfo(oldObj)
			name, newSecretRefs, newConfigMapRefs := extractControllerDeploymentInfo(newObj)

			if !sets.New(oldSecretRefs...).Equal(sets.New(newSecretRefs...)) ||
				!sets.New(oldConfigMapRefs...).Equal(sets.New(newConfigMapRefs...)) {
				g.handleControllerDeploymentCreateOrUpdate(name, newSecretRefs, newConfigMapRefs)
			}
		},
		DeleteFunc: func(obj any) {
			name, secretRefs, configMapRefs := extractControllerDeploymentInfo(obj)
			g.handleControllerDeploymentDelete(name, secretRefs, configMapRefs)
		},
	})
	return err
}

func extractControllerDeploymentInfo(obj any) (string, []string, []string) {
	controllerDeployment, ok := obj.(*gardencorev1.ControllerDeployment)
	if !ok {
		return "", nil, nil
	}

	if controllerDeployment.Helm == nil || controllerDeployment.Helm.OCIRepository == nil {
		return controllerDeployment.Name, nil, nil
	}

	var (
		secretNames     []string
		configMapsNames []string
	)

	if controllerDeployment.Helm.OCIRepository.PullSecretRef != nil {
		secretNames = append(secretNames, controllerDeployment.Helm.OCIRepository.PullSecretRef.Name)
	}
	if controllerDeployment.Helm.OCIRepository.CABundleSecretRef != nil {
		secretNames = append(secretNames, controllerDeployment.Helm.OCIRepository.CABundleSecretRef.Name)
	}

	for _, resource := range controllerDeployment.Resources {
		switch resource.ResourceRef.Kind {
		case "Secret":
			secretNames = append(secretNames, resource.ResourceRef.Name)
		case "ConfigMap":
			configMapsNames = append(configMapsNames, resource.ResourceRef.Name)
		}
	}

	return controllerDeployment.Name, secretNames, configMapsNames
}

func (g *graph) handleControllerDeploymentCreateOrUpdate(controllerDeploymentName string, secretNames, configMapNames []string) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ControllerDeployment", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()

	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeControllerDeployment, "", controllerDeploymentName)
	g.deleteAllIncomingEdges(VertexTypeConfigMap, VertexTypeControllerDeployment, "", controllerDeploymentName)

	controllerDeploymentVertex := g.getOrCreateVertex(VertexTypeControllerDeployment, "", controllerDeploymentName)
	for _, secretName := range secretNames {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, v1beta1constants.GardenNamespace, secretName)
		g.addEdge(secretVertex, controllerDeploymentVertex)
	}
	for _, configMapName := range configMapNames {
		configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, v1beta1constants.GardenNamespace, configMapName)
		g.addEdge(configMapVertex, controllerDeploymentVertex)
	}
}

func (g *graph) handleControllerDeploymentDelete(controllerDeploymentName string, _, _ []string) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ControllerDeployment", "Delete").Observe(time.Since(start).Seconds())
	}()

	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeControllerDeployment, "", controllerDeploymentName)
}
