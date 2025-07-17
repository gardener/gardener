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

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

func (g *graph) setupCredentialsBindingWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			credentialsBinding, ok := obj.(*securityv1alpha1.CredentialsBinding)
			if !ok {
				return
			}
			g.handleCredentialsBindingCreateOrUpdate(credentialsBinding)
		},

		UpdateFunc: func(oldObj, newObj any) {
			oldCredentialsBinding, ok := oldObj.(*securityv1alpha1.CredentialsBinding)
			if !ok {
				return
			}

			newCredentialsBinding, ok := newObj.(*securityv1alpha1.CredentialsBinding)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldCredentialsBinding.CredentialsRef, newCredentialsBinding.CredentialsRef) {
				g.handleCredentialsBindingCreateOrUpdate(newCredentialsBinding)
			}
		},

		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			credentialsBinding, ok := obj.(*securityv1alpha1.CredentialsBinding)
			if !ok {
				return
			}
			g.handleCredentialsBindingDelete(credentialsBinding)
		},
	})
	return err
}

func (g *graph) handleCredentialsBindingCreateOrUpdate(credentialsBinding *securityv1alpha1.CredentialsBinding) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("CredentialsBinding", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeCredentialsBinding, credentialsBinding.Namespace, credentialsBinding.Name)
	g.deleteAllIncomingEdges(VertexTypeWorkloadIdentity, VertexTypeCredentialsBinding, credentialsBinding.Namespace, credentialsBinding.Name)

	var (
		credentialsBindingVertex = g.getOrCreateVertex(VertexTypeCredentialsBinding, credentialsBinding.Namespace, credentialsBinding.Name)
		credentialsVertex        *vertex
	)
	if credentialsBinding.CredentialsRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() &&
		credentialsBinding.CredentialsRef.Kind == "WorkloadIdentity" {
		credentialsVertex = g.getOrCreateVertex(VertexTypeWorkloadIdentity, credentialsBinding.CredentialsRef.Namespace, credentialsBinding.CredentialsRef.Name)
	} else {
		credentialsVertex = g.getOrCreateVertex(VertexTypeSecret, credentialsBinding.CredentialsRef.Namespace, credentialsBinding.CredentialsRef.Name)
	}
	g.addEdge(credentialsVertex, credentialsBindingVertex)
}

func (g *graph) handleCredentialsBindingDelete(credentialsBinding *securityv1alpha1.CredentialsBinding) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("CredentialsBinding", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeCredentialsBinding, credentialsBinding.Namespace, credentialsBinding.Name)
}
