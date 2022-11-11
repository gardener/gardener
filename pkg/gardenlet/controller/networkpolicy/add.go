// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/hostnameresolver"
)

// ControllerName is the name of this controller.
const ControllerName = "networkpolicy"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, seedCluster cluster.Cluster) error {
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.ShootNamespaceSelector == nil {
		r.ShootNamespaceSelector = labels.SelectorFromSet(labels.Set{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
		})
	}
	if r.Resolver == nil {
		resolver, err := hostnameresolver.CreateForCluster(seedCluster.GetConfig(), mgr.GetLogger())
		if err != nil {
			return fmt.Errorf("failed to get hostnameresolver: %w", err)
		}
		r.Resolver = resolver
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Namespace{}).
		Watches(
			source.NewKindWithCache(&corev1.Endpoints{}, seedCluster.GetCache()),
			mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapToNamespaces), mapper.UpdateWithNew, mgr.GetLogger()),
			builder.WithPredicates(r.IsKubernetesEndpoint()),
		).
		Watches(
			source.NewKindWithCache(&networkingv1.NetworkPolicy{}, seedCluster.GetCache()),
			mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapObjectToNamespace), mapper.UpdateWithNew, mgr.GetLogger()),
			builder.WithPredicates(r.IsAllowToSeedApiserverNetworkPolicy()),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.Config.ConcurrentSyncs,
			RecoverPanic:            true,
			RateLimiter:             workqueue.DefaultControllerRateLimiter(),
		}).
		Complete(r)
}

// MapToNamespaces is a mapper function which returns requests for all shoot namespaces + garden namespace + istio-system namespace.
func (r *Reconciler) MapToNamespaces(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
	namespaces := &corev1.NamespaceList{}
	if err := r.SeedClient.List(ctx, namespaces, &client.ListOptions{
		LabelSelector: r.ShootNamespaceSelector,
	}); err != nil {
		log.Error(err, "Unable to list Shoot namespace for updating NetworkPolicy", "networkPolicyName", helper.AllowToSeedAPIServer)
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}
	for _, namespace := range namespaces.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&namespace)})
	}
	requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: v1beta1constants.GardenNamespace}})
	requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: v1beta1constants.IstioSystemNamespace}})

	return requests
}

// MapObjectToNamespace is a mapper function which maps an object to its namespace.
func (r *Reconciler) MapObjectToNamespace(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetNamespace()}}}
}

// IsAllowToSeedApiserverNetworkPolicy returns a predicate which evaluates if the object the allow-to-seed-apiserver network policy.
func (r *Reconciler) IsAllowToSeedApiserverNetworkPolicy() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		policy, ok := obj.(*networkingv1.NetworkPolicy)
		if !ok {
			return false
		}
		return policy.Name == helper.AllowToSeedAPIServer
	})
}

// IsKubernetesEndpoint returns a predicate which evaluates if the object is the kubernetes endpoint.
func (r *Reconciler) IsKubernetesEndpoint() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		endpoints, ok := obj.(*corev1.Endpoints)
		if !ok {
			return false
		}
		return endpoints.Namespace == corev1.NamespaceDefault && endpoints.Name == "kubernetes"
	})
}
