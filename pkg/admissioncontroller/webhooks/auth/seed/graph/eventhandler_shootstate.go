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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupShootStateWatch(informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			partialObjectMeta, ok := obj.(*metav1.PartialObjectMetadata)
			if !ok {
				return
			}
			g.handleShootStateCreate(partialObjectMeta)
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			partialObjectMeta, ok := obj.(*metav1.PartialObjectMetadata)
			if !ok {
				return
			}
			g.handleShootStateDelete(partialObjectMeta)
		},
	})
}

func (g *graph) handleShootStateCreate(partialObjectMeta *metav1.PartialObjectMetadata) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ShootState", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	var (
		shootStateVertex = g.getOrCreateVertex(VertexTypeShootState, partialObjectMeta.Namespace, partialObjectMeta.Name)
		shootVertex      = g.getOrCreateVertex(VertexTypeShoot, partialObjectMeta.Namespace, partialObjectMeta.Name)
	)

	g.addEdge(shootStateVertex, shootVertex)
}

func (g *graph) handleShootStateDelete(partialObjectMeta *metav1.PartialObjectMetadata) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ShootState", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeShootState, partialObjectMeta.Namespace, partialObjectMeta.Name)
}
