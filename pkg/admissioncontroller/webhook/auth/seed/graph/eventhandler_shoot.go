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
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *graph) setupShootWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			shoot, ok := obj.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}
			g.handleShootCreateOrUpdate(shoot)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldShoot, ok := oldObj.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}

			newShoot, ok := newObj.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldShoot.Spec.SeedName, newShoot.Spec.SeedName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Status.SeedName, newShoot.Status.SeedName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.SecretBindingName, newShoot.Spec.SecretBindingName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.CloudProfileName, newShoot.Spec.CloudProfileName) ||
				v1beta1helper.GetShootAuditPolicyConfigMapName(oldShoot.Spec.Kubernetes.KubeAPIServer) != v1beta1helper.GetShootAuditPolicyConfigMapName(newShoot.Spec.Kubernetes.KubeAPIServer) ||
				!v1beta1helper.ShootDNSProviderSecretNamesEqual(oldShoot.Spec.DNS, newShoot.Spec.DNS) ||
				!v1beta1helper.ShootResourceReferencesEqual(oldShoot.Spec.Resources, newShoot.Spec.Resources) {
				g.handleShootCreateOrUpdate(newShoot)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			shoot, ok := obj.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}
			g.handleShootDelete(shoot)
		},
	})
	return err
}

func (g *graph) handleShootCreateOrUpdate(shoot *gardencorev1beta1.Shoot) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Shoot", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeCloudProfile, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeExposureClass, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeInternalSecret, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeConfigMap, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeNamespace, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeSecretBinding, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeShootState, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllOutgoingEdges(VertexTypeShoot, shoot.Namespace, shoot.Name, VertexTypeSeed)

	var (
		shootVertex        = g.getOrCreateVertex(VertexTypeShoot, shoot.Namespace, shoot.Name)
		namespaceVertex    = g.getOrCreateVertex(VertexTypeNamespace, "", shoot.Namespace)
		cloudProfileVertex = g.getOrCreateVertex(VertexTypeCloudProfile, "", shoot.Spec.CloudProfileName)
	)

	if shoot.Spec.SecretBindingName != nil {
		secretBindingVertex := g.getOrCreateVertex(VertexTypeSecretBinding, shoot.Namespace, *shoot.Spec.SecretBindingName)
		g.addEdge(secretBindingVertex, shootVertex)
	}
	g.addEdge(namespaceVertex, shootVertex)
	g.addEdge(cloudProfileVertex, shootVertex)

	if shoot.Spec.SeedName != nil {
		seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", *shoot.Spec.SeedName)
		g.addEdge(shootVertex, seedVertex)
	}

	if shoot.Status.SeedName != nil {
		seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", *shoot.Status.SeedName)
		g.addEdge(shootVertex, seedVertex)
	}

	if shoot.Spec.ExposureClassName != nil {
		exposureClassVertex := g.getOrCreateVertex(VertexTypeExposureClass, "", *shoot.Spec.ExposureClassName)
		g.addEdge(exposureClassVertex, shootVertex)
	}

	if shoot.Spec.Kubernetes.KubeAPIServer != nil &&
		shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig != nil &&
		shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil &&
		shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, shoot.Namespace, shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name)
		g.addEdge(configMapVertex, shootVertex)
	}

	if shoot.Spec.DNS != nil {
		for _, provider := range shoot.Spec.DNS.Providers {
			if provider.SecretName != nil {
				secretVertex := g.getOrCreateVertex(VertexTypeSecret, shoot.Namespace, *provider.SecretName)
				g.addEdge(secretVertex, shootVertex)
			}
		}
	}

	for _, resource := range shoot.Spec.Resources {
		// only secrets and configMap are supported here
		if resource.ResourceRef.APIVersion == "v1" {
			if resource.ResourceRef.Kind == "Secret" {
				secretVertex := g.getOrCreateVertex(VertexTypeSecret, shoot.Namespace, resource.ResourceRef.Name)
				g.addEdge(secretVertex, shootVertex)
			}
			if resource.ResourceRef.Kind == "ConfigMap" {
				configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, shoot.Namespace, resource.ResourceRef.Name)
				g.addEdge(configMapVertex, shootVertex)
			}
		}
	}

	// Those secrets are not directly referenced in the shoot spec, however, they will be created/updated as part of the
	// gardenlet reconciliation and are bound to the lifetime of the shoot, so let's add them here.
	for _, suffix := range gardenerutils.GetShootProjectSecretSuffixes() {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, shoot.Namespace, gardenerutils.ComputeShootProjectResourceName(shoot.Name, suffix))
		g.addEdge(secretVertex, shootVertex)
	}

	// Those internal secrets are not directly referenced in the shoot spec, however, they will be created/updated as part of the
	// gardenlet reconciliation and are bound to the lifetime of the shoot, so let's add them here.
	for _, suffix := range gardenerutils.GetShootProjectInternalSecretSuffixes() {
		secretVertex := g.getOrCreateVertex(VertexTypeInternalSecret, shoot.Namespace, gardenerutils.ComputeShootProjectResourceName(shoot.Name, suffix))
		g.addEdge(secretVertex, shootVertex)
	}

	// Those config maps are not directly referenced in the shoot spec, however, they will be created/updated as part of the
	// gardenlet reconciliation and are bound to the lifetime of the shoot, so let's add them here.
	for _, suffix := range gardenerutils.GetShootProjectConfigMapSuffixes() {
		configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, shoot.Namespace, gardenerutils.ComputeShootProjectResourceName(shoot.Name, suffix))
		g.addEdge(configMapVertex, shootVertex)
	}

	// Similarly, ShootStates are not directly referenced in the shoot spec, however, they will be created/updated/
	// deleted as part of the gardenlet reconciliation and are bound to the lifetime of the shoot as well.
	shootStateVertex := g.getOrCreateVertex(VertexTypeShootState, shoot.Namespace, shoot.Name)
	g.addEdge(shootStateVertex, shootVertex)
}

func (g *graph) handleShootDelete(shoot *gardencorev1beta1.Shoot) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Shoot", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeShoot, shoot.Namespace, shoot.Name)
}
