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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	coordinationv1 "k8s.io/api/coordination/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupLeaseWatch(_ context.Context, informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			lease, ok := obj.(*coordinationv1.Lease)
			if !ok {
				return
			}

			if lease.Namespace != gardencorev1beta1.GardenerSeedLeaseNamespace {
				return
			}

			g.handleLeaseCreate(lease)
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			lease, ok := obj.(*coordinationv1.Lease)
			if !ok {
				return
			}

			if lease.Namespace != gardencorev1beta1.GardenerSeedLeaseNamespace {
				return
			}

			g.handleLeaseDelete(lease)
		},
	})
}

func (g *graph) handleLeaseCreate(lease *coordinationv1.Lease) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Lease", "Create").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	var (
		leaseVertex = g.getOrCreateVertex(VertexTypeLease, lease.Namespace, lease.Name)
		seedVertex  = g.getOrCreateVertex(VertexTypeSeed, "", lease.Name)
	)

	g.addEdge(leaseVertex, seedVertex)
}

func (g *graph) handleLeaseDelete(lease *coordinationv1.Lease) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Lease", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeLease, lease.Namespace, lease.Name)
}
