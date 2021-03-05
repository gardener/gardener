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

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"

	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupManagedSeedWatch(informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}
			g.handleManagedSeedCreateOrUpdate(managedSeed)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldManagedSeed, ok := oldObj.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			newManagedSeed, ok := newObj.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			if oldManagedSeed.Spec.Shoot.Name != newManagedSeed.Spec.Shoot.Name {
				g.handleManagedSeedCreateOrUpdate(newManagedSeed)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}
			g.handleManagedSeedDelete(managedSeed)
		},
	})
}

func (g *graph) handleManagedSeedCreateOrUpdate(managedSeed *seedmanagementv1alpha1.ManagedSeed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ManagedSeed", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)

	var (
		managedSeedVertex = g.getOrCreateVertex(VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)
		shootVertex       = g.getOrCreateVertex(VertexTypeShoot, managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
	)

	g.addEdge(managedSeedVertex, shootVertex)
}

func (g *graph) handleManagedSeedDelete(managedSeed *seedmanagementv1alpha1.ManagedSeed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ManagedSeed", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)
}
