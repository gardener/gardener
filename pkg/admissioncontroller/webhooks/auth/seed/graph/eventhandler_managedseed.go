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

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
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

			oldSeedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(oldManagedSeed)
			if err != nil {
				return
			}
			newSeedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(newManagedSeed)
			if err != nil {
				return
			}

			if oldManagedSeed.Spec.Shoot.Name != newManagedSeed.Spec.Shoot.Name ||
				!gardencorev1beta1helper.SeedBackupSecretRefEqual(oldSeedTemplate.Spec.Backup, newSeedTemplate.Spec.Backup) ||
				!apiequality.Semantic.DeepEqual(oldSeedTemplate.Spec.SecretRef, newSeedTemplate.Spec.SecretRef) {
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

	seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed)
	if err != nil {
		return
	}

	if seedTemplate != nil {
		if seedTemplate.Spec.Backup != nil {
			secretVertex := g.getOrCreateVertex(VertexTypeSecret, seedTemplate.Spec.Backup.SecretRef.Namespace, seedTemplate.Spec.Backup.SecretRef.Name)
			g.addEdge(secretVertex, managedSeedVertex)
		}

		if seedTemplate.Spec.SecretRef != nil {
			secretVertex := g.getOrCreateVertex(VertexTypeSecret, seedTemplate.Spec.SecretRef.Namespace, seedTemplate.Spec.SecretRef.Name)
			g.addEdge(secretVertex, managedSeedVertex)
		}
	}
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
