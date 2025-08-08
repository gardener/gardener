// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func (g *graph) setupProjectWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			project, ok := obj.(*gardencorev1beta1.Project)
			if !ok {
				return
			}
			g.handleProjectCreateOrUpdate(project)
		},

		UpdateFunc: func(oldObj, newObj any) {
			oldProject, ok := oldObj.(*gardencorev1beta1.Project)
			if !ok {
				return
			}

			newProject, ok := newObj.(*gardencorev1beta1.Project)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldProject.Spec.Namespace, newProject.Spec.Namespace) {
				g.handleProjectCreateOrUpdate(newProject)
			}
		},

		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			project, ok := obj.(*gardencorev1beta1.Project)
			if !ok {
				return
			}
			g.handleProjectDelete(project)
		},
	})
	return err
}

func (g *graph) handleProjectCreateOrUpdate(project *gardencorev1beta1.Project) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Project", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeProject, "", project.Name)

	if project.Spec.Namespace != nil {
		var (
			projectVertex   = g.getOrCreateVertex(VertexTypeProject, "", project.Name)
			namespaceVertex = g.getOrCreateVertex(VertexTypeNamespace, "", *project.Spec.Namespace)
		)

		g.addEdge(projectVertex, namespaceVertex)
	}
}

func (g *graph) handleProjectDelete(project *gardencorev1beta1.Project) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Project", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeProject, "", project.Name)
}
