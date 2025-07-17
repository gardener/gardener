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

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

func (g *graph) setupBastionWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			bastion, ok := obj.(*operationsv1alpha1.Bastion)
			if !ok {
				return
			}
			g.handleBastionCreateOrUpdate(bastion)
		},

		UpdateFunc: func(oldObj, newObj any) {
			oldBastion, ok := oldObj.(*operationsv1alpha1.Bastion)
			if !ok {
				return
			}

			newBastion, ok := newObj.(*operationsv1alpha1.Bastion)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldBastion.Spec.SeedName, newBastion.Spec.SeedName) {
				g.handleBastionCreateOrUpdate(newBastion)
			}
		},

		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			bastion, ok := obj.(*operationsv1alpha1.Bastion)
			if !ok {
				return
			}
			g.handleBastionDelete(bastion)
		},
	})
	return err
}

func (g *graph) handleBastionCreateOrUpdate(bastion *operationsv1alpha1.Bastion) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Bastion", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeBastion, bastion.Namespace, bastion.Name)

	if bastion.Spec.SeedName != nil {
		bastionVertex := g.getOrCreateVertex(VertexTypeBastion, bastion.Namespace, bastion.Name)
		seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", *bastion.Spec.SeedName)
		g.addEdge(bastionVertex, seedVertex)
	}
}

func (g *graph) handleBastionDelete(bastion *operationsv1alpha1.Bastion) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Bastion", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeBastion, bastion.Namespace, bastion.Name)
}
