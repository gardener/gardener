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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (g *graph) setupSeedWatch(ctx context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			seed, ok := obj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}
			g.handleSeedCreateOrUpdate(seed)
			g.handleManagedSeedIfSeedBelongsToIt(ctx, seed.Name)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}

			newSeed, ok := newObj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldSeed.Spec.SecretRef, newSeed.Spec.SecretRef) ||
				!v1beta1helper.SeedBackupSecretRefEqual(oldSeed.Spec.Backup, newSeed.Spec.Backup) ||
				!seedDNSProviderSecretRefEqual(oldSeed.Spec.DNS.Provider, newSeed.Spec.DNS.Provider) {
				g.handleSeedCreateOrUpdate(newSeed)
			}

			newGardenletReadyCondition := v1beta1helper.GetCondition(newSeed.Status.Conditions, gardencorev1beta1.SeedGardenletReady)

			// When the GardenletReady condition transitions to 'Unknown' then the client certificate might be expired.
			// Hence, check if seed belongs to a ManagedSeed and reconcile it to potentially allow re-bootstrapping it.
			if (newGardenletReadyCondition != nil && newGardenletReadyCondition.Status == gardencorev1beta1.ConditionUnknown) ||
				// When the client certificate expiration timestamp changes then we check if seed belongs to a ManagedSeed
				// and reconcile it to potentially forbid to bootstrap it again.
				!apiequality.Semantic.DeepEqual(oldSeed.Status.ClientCertificateExpirationTimestamp, newSeed.Status.ClientCertificateExpirationTimestamp) {

				g.handleManagedSeedIfSeedBelongsToIt(ctx, newSeed.Name)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			seed, ok := obj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}
			g.handleSeedDelete(seed)
		},
	})
	return err
}

func (g *graph) handleSeedCreateOrUpdate(seed *gardencorev1beta1.Seed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Seed", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeSeed, "", seed.Name)
	g.deleteAllIncomingEdges(VertexTypeNamespace, VertexTypeSeed, "", seed.Name)
	g.deleteAllIncomingEdges(VertexTypeLease, VertexTypeSeed, "", seed.Name)
	g.deleteAllIncomingEdges(VertexTypeConfigMap, VertexTypeSeed, "", seed.Name)

	seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", seed.Name)
	namespaceVertex := g.getOrCreateVertex(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed.Name))
	g.addEdge(namespaceVertex, seedVertex)

	configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, metav1.NamespaceSystem, v1beta1constants.ClusterIdentity)
	g.addEdge(configMapVertex, seedVertex)

	leaseVertex := g.getOrCreateVertex(VertexTypeLease, gardencorev1beta1.GardenerSeedLeaseNamespace, seed.Name)
	g.addEdge(leaseVertex, seedVertex)

	if seed.Spec.SecretRef != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}

	if seed.Spec.Backup != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.Backup.SecretRef.Namespace, seed.Spec.Backup.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}

	if seed.Spec.DNS.Provider != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.DNS.Provider.SecretRef.Namespace, seed.Spec.DNS.Provider.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}
}

func (g *graph) handleSeedDelete(seed *gardencorev1beta1.Seed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Seed", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeSeed, "", seed.Name)
}

func (g *graph) handleManagedSeedIfSeedBelongsToIt(ctx context.Context, seedName string) {
	// error is ignored here since we cannot do anything meaningful with it
	if managedSeed, err := kubernetesutils.GetManagedSeedByName(ctx, g.client, seedName); err == nil && managedSeed != nil {
		g.handleManagedSeedCreateOrUpdate(ctx, managedSeed)
	}
}

func seedDNSProviderSecretRefEqual(oldDNS, newDNS *gardencorev1beta1.SeedDNSProvider) bool {
	if oldDNS == nil && newDNS == nil {
		return true
	}

	if oldDNS != nil && newDNS != nil {
		return apiequality.Semantic.DeepEqual(oldDNS.SecretRef, newDNS.SecretRef)
	}

	return false
}
