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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func (g *graph) setupProjectWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			project, ok := obj.(*gardencorev1beta1.Project)
			if !ok {
				return
			}
			g.handleProjectCreateOrUpdate(project)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
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

		DeleteFunc: func(obj interface{}) {
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
