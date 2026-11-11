// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

			g.handleServiceAccountCreate(serviceAccount)
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

func (g *graph) handleServiceAccountCreate(serviceAccount *corev1.ServiceAccount) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("ServiceAccount", "Create").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	// Only self-hosted shoots derive an edge from ServiceAccounts (ServiceAccount -> Shoot for extension garden
	// access). In the garden cluster, the gardenlet bootstrap flow uses the `TokenRequest` API and no longer relies on
	// ServiceAccount token `Secret`s referenced via `.secrets[]` (auto-population was dropped in Kubernetes 1.24), so
	// there is nothing to model here.
	if !g.forSelfHostedShoots {
		return
	}

	shootName, ok := gardenerutils.ParseExtensionShootServiceAccountName(serviceAccount.Name)
	if !ok {
		return
	}

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
