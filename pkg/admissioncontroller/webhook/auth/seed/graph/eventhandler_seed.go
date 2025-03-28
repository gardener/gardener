// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
		AddFunc: func(obj any) {
			seed, ok := obj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}
			g.handleSeedCreateOrUpdate(seed)
			g.handleManagedSeedIfSeedBelongsToIt(ctx, seed.Name)
		},

		UpdateFunc: func(oldObj, newObj any) {
			oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}

			newSeed, ok := newObj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}

			if !v1beta1helper.SeedBackupSecretRefEqual(oldSeed.Spec.Backup, newSeed.Spec.Backup) ||
				!v1beta1helper.ResourceReferencesEqual(oldSeed.Spec.Resources, newSeed.Spec.Resources) ||
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

		DeleteFunc: func(obj any) {
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

	if seed.Spec.Backup != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.Backup.SecretRef.Namespace, seed.Spec.Backup.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}

	if seed.Spec.DNS.Provider != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.DNS.Provider.SecretRef.Namespace, seed.Spec.DNS.Provider.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}

	for _, resource := range seed.Spec.Resources {
		// only secrets and configMap are supported here
		if resource.ResourceRef.APIVersion == "v1" {
			if resource.ResourceRef.Kind == "Secret" {
				secretVertex := g.getOrCreateVertex(VertexTypeSecret, v1beta1constants.GardenNamespace, resource.ResourceRef.Name)
				g.addEdge(secretVertex, seedVertex)
			}
			if resource.ResourceRef.Kind == "ConfigMap" {
				configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, v1beta1constants.GardenNamespace, resource.ResourceRef.Name)
				g.addEdge(configMapVertex, seedVertex)
			}
		}
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
