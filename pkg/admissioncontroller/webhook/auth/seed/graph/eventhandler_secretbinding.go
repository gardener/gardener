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

func (g *graph) setupSecretBindingWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secretBinding, ok := obj.(*gardencorev1beta1.SecretBinding)
			if !ok {
				return
			}
			g.handleSecretBindingCreateOrUpdate(secretBinding)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSecretBinding, ok := oldObj.(*gardencorev1beta1.SecretBinding)
			if !ok {
				return
			}

			newSecretBinding, ok := newObj.(*gardencorev1beta1.SecretBinding)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldSecretBinding.SecretRef, newSecretBinding.SecretRef) {
				g.handleSecretBindingCreateOrUpdate(newSecretBinding)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			secretBinding, ok := obj.(*gardencorev1beta1.SecretBinding)
			if !ok {
				return
			}
			g.handleSecretBindingDelete(secretBinding)
		},
	})
	return err
}

func (g *graph) handleSecretBindingCreateOrUpdate(secretBinding *gardencorev1beta1.SecretBinding) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("SecretBinding", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeSecretBinding, secretBinding.Namespace, secretBinding.Name)

	var (
		secretBindingVertex = g.getOrCreateVertex(VertexTypeSecretBinding, secretBinding.Namespace, secretBinding.Name)
		secretVertex        = g.getOrCreateVertex(VertexTypeSecret, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
	)

	g.addEdge(secretVertex, secretBindingVertex)
}

func (g *graph) handleSecretBindingDelete(secretBinding *gardencorev1beta1.SecretBinding) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("SecretBinding", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeSecretBinding, secretBinding.Namespace, secretBinding.Name)
}
