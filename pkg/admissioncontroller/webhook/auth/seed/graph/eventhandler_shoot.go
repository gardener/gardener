// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *graph) setupShootWatch(ctx context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			shoot, ok := obj.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}
			g.handleShootCreateOrUpdate(ctx, shoot)
		},

		UpdateFunc: func(oldObj, newObj any) {
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
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.CredentialsBindingName, newShoot.Spec.CredentialsBindingName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.CloudProfileName, newShoot.Spec.CloudProfileName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.CloudProfile, newShoot.Spec.CloudProfile) ||
				v1beta1helper.GetShootAuditPolicyConfigMapName(oldShoot.Spec.Kubernetes.KubeAPIServer) != v1beta1helper.GetShootAuditPolicyConfigMapName(newShoot.Spec.Kubernetes.KubeAPIServer) ||
				v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(oldShoot.Spec.Kubernetes.KubeAPIServer) != v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(newShoot.Spec.Kubernetes.KubeAPIServer) ||
				!apiequality.Semantic.DeepEqual(v1beta1helper.GetShootAuthorizationConfiguration(oldShoot.Spec.Kubernetes.KubeAPIServer), v1beta1helper.GetShootAuthorizationConfiguration(newShoot.Spec.Kubernetes.KubeAPIServer)) ||
				!v1beta1helper.ShootDNSProviderSecretNamesEqual(oldShoot.Spec.DNS, newShoot.Spec.DNS) ||
				!v1beta1helper.ResourceReferencesEqual(oldShoot.Spec.Resources, newShoot.Spec.Resources) ||
				v1beta1helper.HasManagedIssuer(oldShoot) != v1beta1helper.HasManagedIssuer(newShoot) ||
				!g.hasExpectedShootBindingEdges(newShoot) {
				g.handleShootCreateOrUpdate(ctx, newShoot)
			}
		},

		DeleteFunc: func(obj any) {
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

func (g *graph) hasExpectedShootBindingEdges(shoot *gardencorev1beta1.Shoot) bool {
	g.lock.RLock()
	defer g.lock.RUnlock()

	hasEdge := func(vertexType VertexType, bindingName *string) bool {
		if bindingName == nil {
			return true // if bindingName is nil, there is no edge to target vertex, hence we assume the edge exists
		}
		shootVertex, foundShootVertex := g.getVertex(VertexTypeShoot, shoot.Namespace, shoot.Name)
		bindingVertex, foundBindingVertex := g.getVertex(vertexType, shoot.Namespace, *bindingName)
		return foundShootVertex && foundBindingVertex && g.graph.HasEdgeFromTo(bindingVertex.id, shootVertex.id)
	}
	return hasEdge(VertexTypeSecretBinding, shoot.Spec.SecretBindingName) && hasEdge(VertexTypeCredentialsBinding, shoot.Spec.CredentialsBindingName)
}

func (g *graph) handleShootCreateOrUpdate(ctx context.Context, shoot *gardencorev1beta1.Shoot) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Shoot", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeCloudProfile, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeNamespacedCloudProfile, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeExposureClass, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeInternalSecret, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeConfigMap, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeNamespace, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeSecretBinding, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeCredentialsBinding, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllIncomingEdges(VertexTypeShootState, VertexTypeShoot, shoot.Namespace, shoot.Name)
	g.deleteAllOutgoingEdges(VertexTypeShoot, shoot.Namespace, shoot.Name, VertexTypeSeed)

	var (
		shootVertex     = g.getOrCreateVertex(VertexTypeShoot, shoot.Namespace, shoot.Name)
		namespaceVertex = g.getOrCreateVertex(VertexTypeNamespace, "", shoot.Namespace)
	)

	if shoot.Spec.SecretBindingName != nil {
		secretBindingVertex := g.getOrCreateVertex(VertexTypeSecretBinding, shoot.Namespace, *shoot.Spec.SecretBindingName)
		g.addEdge(secretBindingVertex, shootVertex)
	}

	if shoot.Spec.CredentialsBindingName != nil {
		credentialsBindingVertex := g.getOrCreateVertex(VertexTypeCredentialsBinding, shoot.Namespace, *shoot.Spec.CredentialsBindingName)
		g.addEdge(credentialsBindingVertex, shootVertex)
	}

	g.addEdge(namespaceVertex, shootVertex)

	cloudProfileReference := gardenerutils.BuildV1beta1CloudProfileReference(shoot)
	if cloudProfileReference != nil {
		var cloudProfileVertex *vertex
		switch cloudProfileReference.Kind {
		case v1beta1constants.CloudProfileReferenceKindCloudProfile:
			cloudProfileVertex = g.getOrCreateVertex(VertexTypeCloudProfile, "", cloudProfileReference.Name)
		case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
			cloudProfileVertex = g.getOrCreateVertex(VertexTypeNamespacedCloudProfile, shoot.Namespace, cloudProfileReference.Name)
		}
		g.addEdge(cloudProfileVertex, shootVertex)
	}

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

	if kubeAPIServer := shoot.Spec.Kubernetes.KubeAPIServer; kubeAPIServer != nil {
		if configMapName := v1beta1helper.GetShootAuditPolicyConfigMapName(kubeAPIServer); len(configMapName) > 0 {
			configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, shoot.Namespace, configMapName)
			g.addEdge(configMapVertex, shootVertex)
		}

		if configMapName := v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(kubeAPIServer); len(configMapName) > 0 {
			configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, shoot.Namespace, configMapName)
			g.addEdge(configMapVertex, shootVertex)
		}

		if configMapName := v1beta1helper.GetShootAuthorizationConfigurationConfigMapName(kubeAPIServer); len(configMapName) > 0 {
			configMapVertex := g.getOrCreateVertex(VertexTypeConfigMap, shoot.Namespace, configMapName)
			g.addEdge(configMapVertex, shootVertex)
		}

		if kubeAPIServer.StructuredAuthorization != nil {
			for _, kubeconfig := range kubeAPIServer.StructuredAuthorization.Kubeconfigs {
				secretVertex := g.getOrCreateVertex(VertexTypeSecret, shoot.Namespace, kubeconfig.SecretName)
				g.addEdge(secretVertex, shootVertex)
			}
		}
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

	if v1beta1helper.HasManagedIssuer(shoot) {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: shoot.Namespace}}
		if err := g.client.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err == nil {
			if projectName, ok := namespace.Labels[v1beta1constants.ProjectName]; ok {
				saPublicKeysSecretName := gardenerutils.ComputeManagedShootIssuerSecretName(projectName, shoot.UID)
				saPublicKeysSecretVertex := g.getOrCreateVertex(VertexTypeSecret, gardencorev1beta1.GardenerShootIssuerNamespace, saPublicKeysSecretName)
				g.addEdge(saPublicKeysSecretVertex, shootVertex)
			}
		}
	}
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
