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
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupManagedSeedWatch(ctx context.Context, informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}
			g.handleManagedSeedCreateOrUpdate(ctx, managedSeed)
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
				!apiequality.Semantic.DeepEqual(oldSeedTemplate.Spec.SecretRef, newSeedTemplate.Spec.SecretRef) ||
				!gardenletBootstrapModeEqual(oldManagedSeed.Spec.Gardenlet, newManagedSeed.Spec.Gardenlet) {
				g.handleManagedSeedCreateOrUpdate(ctx, newManagedSeed)
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

func (g *graph) handleManagedSeedCreateOrUpdate(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ManagedSeed", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)
	g.deleteAllIncomingEdges(VertexTypeServiceAccount, VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)
	g.deleteAllIncomingEdges(VertexTypeClusterRoleBinding, VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)
	g.deleteAllOutgoingEdges(VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name, VertexTypeShoot)

	var (
		managedSeedVertex = g.getOrCreateVertex(VertexTypeManagedSeed, managedSeed.Namespace, managedSeed.Name)
		shootVertex       = g.getOrCreateVertex(VertexTypeShoot, managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
	)

	g.addEdge(managedSeedVertex, shootVertex)

	seedTemplate, gardenletConfig, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed)
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

	if gardenletConfig != nil {
		if err := g.client.Get(ctx, kutil.Key(managedSeed.Name), &gardencorev1beta1.Seed{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return
			}

			if managedSeed.Spec.Gardenlet.Bootstrap == nil {
				return
			}

			switch *managedSeed.Spec.Gardenlet.Bootstrap {
			case seedmanagementv1alpha1.BootstrapToken:
				secretVertex := g.getOrCreateVertex(VertexTypeSecret, metav1.NamespaceSystem, bootstraptokenapi.BootstrapTokenSecretPrefix+bootstraputil.TokenID(managedSeed.ObjectMeta))
				g.addEdge(secretVertex, managedSeedVertex)

			case seedmanagementv1alpha1.BootstrapServiceAccount:
				var (
					serviceAccountName     = bootstraputil.ServiceAccountName(managedSeed.Name)
					clusterRoleBindingName = bootstraputil.ClusterRoleBindingName(managedSeed.Namespace, serviceAccountName)

					serviceAccountVertex     = g.getOrCreateVertex(VertexTypeServiceAccount, managedSeed.Namespace, serviceAccountName)
					clusterRoleBindingVertex = g.getOrCreateVertex(VertexTypeClusterRoleBinding, "", clusterRoleBindingName)
				)

				g.addEdge(serviceAccountVertex, managedSeedVertex)
				g.addEdge(clusterRoleBindingVertex, managedSeedVertex)
			}
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

func gardenletBootstrapModeEqual(oldGardenlet, newGardenlet *seedmanagementv1alpha1.Gardenlet) bool {
	if oldGardenlet == nil && newGardenlet == nil {
		return true
	}

	if oldGardenlet != nil && newGardenlet != nil {
		return apiequality.Semantic.DeepEqual(oldGardenlet.Bootstrap, newGardenlet.Bootstrap)
	}

	return false
}
