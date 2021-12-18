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

package graph

import (
	"context"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupShootLeftoverWatch(_ context.Context, informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			shootLeftover, ok := obj.(*gardencorev1alpha1.ShootLeftover)
			if !ok {
				return
			}
			g.handleShootLeftoverCreateOrUpdate(shootLeftover)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldShootLeftover, ok := oldObj.(*gardencorev1alpha1.ShootLeftover)
			if !ok {
				return
			}

			newShootLeftover, ok := newObj.(*gardencorev1alpha1.ShootLeftover)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldShootLeftover.Spec.SeedName, newShootLeftover.Spec.SeedName) ||
				!apiequality.Semantic.DeepEqual(oldShootLeftover.Spec.ShootName, newShootLeftover.Spec.ShootName) {
				g.handleShootLeftoverCreateOrUpdate(newShootLeftover)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			shootLeftover, ok := obj.(*gardencorev1alpha1.ShootLeftover)
			if !ok {
				return
			}
			g.handleShootLeftoverDelete(shootLeftover)
		},
	})
}

func (g *graph) handleShootLeftoverCreateOrUpdate(shootLeftover *gardencorev1alpha1.ShootLeftover) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ShootLeftover", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllOutgoingEdges(VertexTypeShootLeftover, shootLeftover.Namespace, shootLeftover.Name, VertexTypeSeed)
	g.deleteAllOutgoingEdges(VertexTypeShootLeftover, shootLeftover.Namespace, shootLeftover.Name, VertexTypeShoot)

	var (
		shootLeftoverVertex = g.getOrCreateVertex(VertexTypeShootLeftover, shootLeftover.Namespace, shootLeftover.Name)
		seedVertex          = g.getOrCreateVertex(VertexTypeSeed, "", shootLeftover.Spec.SeedName)
		shootVertex         = g.getOrCreateVertex(VertexTypeShoot, shootLeftover.Namespace, shootLeftover.Spec.ShootName)
	)

	g.addEdge(shootLeftoverVertex, seedVertex)
	g.addEdge(shootLeftoverVertex, shootVertex)
}

func (g *graph) handleShootLeftoverDelete(shootLeftover *gardencorev1alpha1.ShootLeftover) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ShootLeftover", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeShootLeftover, shootLeftover.Namespace, shootLeftover.Name)
}
