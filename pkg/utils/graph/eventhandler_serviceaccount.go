// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
)

func (g *graph) setupServiceAccountWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			serviceAccount, ok := obj.(*corev1.ServiceAccount)
			if !ok {
				return
			}

			if !strings.HasPrefix(serviceAccount.Name, gardenletbootstraputil.ServiceAccountNamePrefix) &&
				!strings.HasPrefix(serviceAccount.Name, v1beta1constants.ExtensionShootServiceAccountPrefix) {
				return
			}

			g.handleServiceAccountCreateOrUpdate(serviceAccount)
		},

		UpdateFunc: func(oldObj, newObj any) {
			oldServiceAccount, ok := oldObj.(*corev1.ServiceAccount)
			if !ok {
				return
			}

			newServiceAccount, ok := newObj.(*corev1.ServiceAccount)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldServiceAccount.Secrets, newServiceAccount.Secrets) {
				g.handleServiceAccountCreateOrUpdate(newServiceAccount)
			}
		},

		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			serviceAccount, ok := obj.(*corev1.ServiceAccount)
			if !ok {
				return
			}

			if !strings.HasPrefix(serviceAccount.Name, gardenletbootstraputil.ServiceAccountNamePrefix) &&
				!strings.HasPrefix(serviceAccount.Name, v1beta1constants.ExtensionShootServiceAccountPrefix) {
				return
			}

			g.handleServiceAccountDelete(serviceAccount)
		},
	})
	return err
}

func (g *graph) handleServiceAccountCreateOrUpdate(serviceAccount *corev1.ServiceAccount) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ServiceAccount", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	if g.forSelfHostedShoots {
		g.handleServiceAccountCreateOrUpdateForShoots(serviceAccount)
		return
	}

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeServiceAccount, serviceAccount.Namespace, serviceAccount.Name)

	serviceAccountVertex := g.getOrCreateVertex(VertexTypeServiceAccount, serviceAccount.Namespace, serviceAccount.Name)

	for _, secret := range serviceAccount.Secrets {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, serviceAccount.Namespace, secret.Name)
		g.addEdge(secretVertex, serviceAccountVertex)
	}
}

func (g *graph) handleServiceAccountCreateOrUpdateForShoots(serviceAccount *corev1.ServiceAccount) {
	if !strings.HasPrefix(serviceAccount.Name, v1beta1constants.ExtensionShootServiceAccountPrefix) {
		return
	}

	// SA name: extension-shoot--<shootName>--<controllerInstallationName>
	withoutPrefix := strings.TrimPrefix(serviceAccount.Name, v1beta1constants.ExtensionShootServiceAccountPrefix)
	shootName, _, found := strings.Cut(withoutPrefix, "--")
	if !found || shootName == "" {
		return
	}

	g.deleteAllOutgoingEdges(VertexTypeServiceAccount, serviceAccount.Namespace, serviceAccount.Name, VertexTypeShoot)

	serviceAccountVertex := g.getOrCreateVertex(VertexTypeServiceAccount, serviceAccount.Namespace, serviceAccount.Name)
	shootVertex := g.getOrCreateVertex(VertexTypeShoot, serviceAccount.Namespace, shootName)
	g.addEdge(serviceAccountVertex, shootVertex)
}

func (g *graph) handleServiceAccountDelete(serviceAccount *corev1.ServiceAccount) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ServiceAccount", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeServiceAccount, serviceAccount.Namespace, serviceAccount.Name)
}
